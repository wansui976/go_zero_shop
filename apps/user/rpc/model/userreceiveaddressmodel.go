package model

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlc"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ UserReceiveAddressModel = (*CustomUserReceiveAddressModel)(nil)

const (
	// TTLs (秒)
	listTTL      = 60 * 5       // 5 minutes for normal list
	emptyListTTL = 60           // 1 minute for empty list
	singleTTL    = 60 * 60      // 1 hour for single address (主键缓存)
	verKeyTTL    = 60 * 60 * 24 // 24 hours for version key

	// key prefixes
	verKeyPrefix  = "user:address:ver:"  // + uid
	listKeyPrefix = "user:address:list:" // + uid + ":" + ver
	idKeyPrefix   = "user:address:id:"   // + id    （与 go-zero 生成 key 保持一致）
)

// CustomUserReceiveAddressModel 扩展 defaultUserReceiveAddressModel，注入 redis Client
type (
	UserReceiveAddressModel interface {
		userReceiveAddressModel
		UpdateIsDelete(ctx context.Context, data *UserReceiveAddress) error
		FindAllByUid(ctx context.Context, uid int64) ([]*UserReceiveAddress, error)
		InsertWithTx(ctx context.Context, sess sqlx.Session, data *UserReceiveAddress) (sql.Result, error)
		CancelOldDefaultAddress(ctx context.Context, sess sqlx.Session, uid int64) error
		ValidateAddressData(data *UserReceiveAddress) error
		IncVersion(ctx context.Context, uid int64) (int64, error)
		GetVersion(ctx context.Context, uid int64) (int64, error)
	}

	CustomUserReceiveAddressModel struct {
		*defaultUserReceiveAddressModel
		redis *redis.Client
	}
)

// NewUserReceiveAddressModel 保留原构造（不使用外部 redis）
func NewUserReceiveAddressModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) UserReceiveAddressModel {
	return &CustomUserReceiveAddressModel{
		defaultUserReceiveAddressModel: newUserReceiveAddressModel(conn, c, opts...),
		redis:                          nil,
	}
}

// NewUserReceiveAddressModelWithRedis 注入 redis client（企业环境推荐使用）
func NewUserReceiveAddressModelWithRedis(conn sqlx.SqlConn, c cache.CacheConf, r *redis.Client, opts ...cache.Option) UserReceiveAddressModel {
	return &CustomUserReceiveAddressModel{
		defaultUserReceiveAddressModel: newUserReceiveAddressModel(conn, c, opts...),
		redis:                          r,
	}
}

// helper keys
func verKey(uid int64) string {
	return fmt.Sprintf("%s%d", verKeyPrefix, uid)
}
func listKey(uid int64, ver int64) string {
	return fmt.Sprintf("%s%d:%d", listKeyPrefix, uid, ver)
}
func idKey(id int64) string {
	return fmt.Sprintf("%s%d", idKeyPrefix, id)
}

// GetVersion: 从 redis 读取版本，若 redis 未注入或 key 不存在则返回 1（安全容错）
func (m *CustomUserReceiveAddressModel) GetVersion(ctx context.Context, uid int64) (int64, error) {
	if uid <= 0 {
		return 1, errors.New("invalid uid")
	}
	if m.redis == nil {
		return 1, nil
	}
	key := verKey(uid)
	v, err := m.redis.Get(ctx, key).Int64()
	if err == redis.Nil {
		return 1, nil
	}
	if err != nil {
		logx.Errorf("GetVersion: redis GET err: key=%s, err=%v", key, err)
		// 容错返回 1（不要因为 redis 问题影响主流程）
		return 1, nil
	}
	if v <= 0 {
		return 1, nil
	}
	return v, nil
}

// IncVersion: 原子 INCR 版本号（使用 redis INCR 保证原子性）
// 返回新版本号
func (m *CustomUserReceiveAddressModel) IncVersion(ctx context.Context, uid int64) (int64, error) {
	if uid <= 0 {
		return 0, errors.New("invalid uid")
	}
	// 如果没有 redis 注入，则回退为删除 list cache（best-effort）并返回 1
	if m.redis == nil {
		// 尝试删除可能的 list cache key(s)（保守删除近似 key：ver=1）
		_ = m.DelCacheCtx(ctx, listKey(uid, 1))
		return 1, nil
	}
	key := verKey(uid)
	newV, err := m.redis.Incr(ctx, key).Result()
	if err != nil {
		logx.Errorf("IncVersion: redis INCR err: key=%s, err=%v", key, err)
		return 0, err
	}
	// 设置 TTL（尽量保证版本 key 不永久存在）
	if err := m.redis.Expire(ctx, key, time.Duration(verKeyTTL)*time.Second).Err(); err != nil {
		// TTL 失败不是致命问题，记录日志
		logx.Errorf("IncVersion: set expire failed key=%s err=%v", key, err)
	}
	return newV, nil
}

// FindAllByUid：优先按 ver 从缓存读取 list，若未命中回源 DB 并写缓存（含空 list）
// 使用 default model 的 QueryRowCtx（带缓存回源机制）
func (m *CustomUserReceiveAddressModel) FindAllByUid(ctx context.Context, uid int64) ([]*UserReceiveAddress, error) {
	if uid <= 0 {
		return nil, errors.New("invalid uid")
	}

	ver, _ := m.GetVersion(ctx, uid)
	key := listKey(uid, ver)

	var resp []*UserReceiveAddress
	// QueryRowCtx: 如果缓存未命中，会执行回源函数并写入 cache（默认 key）
	err := m.QueryRowCtx(ctx, &resp, key, func(ctx context.Context, conn sqlx.SqlConn, v interface{}) error {
		query := fmt.Sprintf("SELECT %s FROM %s WHERE uid = ? AND is_delete = 0 ORDER BY is_default DESC, update_time DESC", userReceiveAddressRows, m.table)
		return conn.QueryRowsCtx(ctx, v.(*[]*UserReceiveAddress), query, uid)
	})
	if err == nil {
		// 成功从 cache 或 回源中拿到结果
		return resp, nil
	}

	// 如果是 cache 未命中或者 cache 层返回 ErrNotFound，直接 fallback 到数据库查询（并写入缓存）
	if errors.Is(err, sqlc.ErrNotFound) || err == sql.ErrNoRows {
		// 直接从 DB 查询并短期缓存空值
		var rows []*UserReceiveAddress
		query := fmt.Sprintf("SELECT %s FROM %s WHERE uid = ? AND is_delete = 0 ORDER BY is_default DESC, update_time DESC", userReceiveAddressRows, m.table)
		if qerr := m.QueryRowsNoCacheCtx(ctx, &rows, query, uid); qerr != nil {
			if qerr == sql.ErrNoRows {
				// 写空 list 短期缓存（避免缓存穿透），使用 SetCacheWithExpire（if available）
				_ = m.SetCacheWithExpire(key, []*UserReceiveAddress{}, time.Second*emptyListTTL)
				return []*UserReceiveAddress{}, nil
			}
			return nil, qerr
		}
		// 写入缓存并返回
		_ = m.SetCacheWithExpire(key, rows, time.Second*listTTL)
		return rows, nil
	}

	// 其他错误直接返回
	return nil, err
}

// UpdateIsDelete：逻辑删除 + 清理主键缓存 + 递增版本使 list 缓存失效
func (m *CustomUserReceiveAddressModel) UpdateIsDelete(ctx context.Context, data *UserReceiveAddress) error {
	if data == nil || data.Id <= 0 {
		return errors.New("invalid data")
	}

	// 1) 校验归属
	var ownerUid int64
	queryOwner := fmt.Sprintf("SELECT uid FROM %s WHERE id = ? LIMIT 1", m.table)
	if err := m.QueryRowNoCacheCtx(ctx, &ownerUid, queryOwner, data.Id); err != nil {
		if err == sql.ErrNoRows {
			return errors.New("address not found")
		}
		return fmt.Errorf("query owner failed: %w", err)
	}
	if data.Uid != 0 && ownerUid != data.Uid {
		return errors.New("forbidden: address not belong to user")
	}

	// 2) 清主键缓存（用模型的 DelCacheCtx）
	primaryCacheKey := idKey(data.Id)
	if err := m.DelCacheCtx(ctx, primaryCacheKey); err != nil {
		// 非致命
		logx.Errorf("UpdateIsDelete: DelCacheCtx primaryKey failed key=%s err=%v", primaryCacheKey, err)
	}

	// 3) 执行逻辑删除
	_, err := m.ExecCtx(ctx, func(ctx context.Context, conn sqlx.SqlConn) (sql.Result, error) {
		q := fmt.Sprintf("UPDATE %s SET is_delete = 1, update_time = now() WHERE id = ?", m.table)
		return conn.ExecCtx(ctx, q, data.Id)
	}, primaryCacheKey)
	if err != nil {
		return fmt.Errorf("db update failed: %w", err)
	}

	// 4) 递增版本号（原子，redis）
	if _, ierr := m.IncVersion(ctx, ownerUid); ierr != nil {
		logx.Errorf("UpdateIsDelete: incVersion failed uid=%d err=%v", ownerUid, ierr)
	}
	return nil
}

var phoneRegex = regexp.MustCompile(`^1[3-9]\d{9}$`)

// InsertWithTx：事务内插入（使用 sqlx.Session）
func (m *CustomUserReceiveAddressModel) InsertWithTx(ctx context.Context, sess sqlx.Session, data *UserReceiveAddress) (sql.Result, error) {
	if err := m.ValidateAddressData(data); err != nil {
		return nil, err
	}
	query := fmt.Sprintf(`
		INSERT INTO %s (id, uid, name, phone, province, city, region, detail_address, is_default, is_delete, create_time, update_time)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW())
	`, m.table)
	return sess.ExecCtx(ctx, query,
		data.Id, data.Uid, data.Name, data.Phone,
		data.Province, data.City, data.Region, data.DetailAddress,
		data.IsDefault, data.IsDelete,
	)
}

// CancelOldDefaultAddress：事务内取消 uid 的旧默认地址（如果存在）
func (m *CustomUserReceiveAddressModel) CancelOldDefaultAddress(ctx context.Context, sess sqlx.Session, uid int64) error {
	if uid <= 0 {
		return errors.New("invalid uid")
	}
	var oldDefaultId int64
	query := fmt.Sprintf("SELECT id FROM %s WHERE uid = ? AND is_default = 1 AND is_delete = 0 LIMIT 1", m.table)
	if err := sess.QueryRowCtx(ctx, &oldDefaultId, query, uid); err != nil {
		if err == sql.ErrNoRows {
			return nil
		}
		return fmt.Errorf("query old default failed: %w", err)
	}

	update := fmt.Sprintf("UPDATE %s SET is_default = 0, update_time = NOW() WHERE id = ? AND uid = ?", m.table)
	res, err := sess.ExecCtx(ctx, update, oldDefaultId, uid)
	if err != nil {
		return fmt.Errorf("update old default failed: %w", err)
	}
	// rowsAffected optional check
	if aff, _ := res.RowsAffected(); aff == 0 {
		logx.Infof("CancelOldDefaultAddress: no rows updated uid=%d oldDefaultId=%d", uid, oldDefaultId)
	}
	// 删除主键缓存（best-effort）
	_ = m.DelCacheCtx(ctx, idKey(oldDefaultId))
	return nil
}

// ValidateAddressData：数据校验
func (m *CustomUserReceiveAddressModel) ValidateAddressData(data *UserReceiveAddress) error {
	if data == nil {
		return errors.New("nil address")
	}
	if data.Uid <= 0 {
		return errors.New("invalid uid")
	}
	if data.Name == "" {
		return errors.New("name empty")
	}
	if !phoneRegex.MatchString(data.Phone) {
		return errors.New("invalid phone")
	}
	if data.Province == "" || data.City == "" || data.Region == "" || data.DetailAddress == "" {
		return errors.New("address incomplete")
	}
	if data.IsDefault != 0 && data.IsDefault != 1 {
		return errors.New("is_default must be 0 or 1")
	}
	return nil
}
