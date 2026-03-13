package model

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlc"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ UserCollectionModel = (*customUserCollectionModel)(nil)

type (
	// UserCollectionModel is an interface to be customized, add more methods here,
	// and implement the added methods in customUserCollectionModel.
	UserCollectionModel interface {
		userCollectionModel
		UpdateIsDelete(ctx context.Context, data *UserCollection) error
		FindAllByUid(ctx context.Context, uid int64) ([]*UserCollection, error)
	}

	customUserCollectionModel struct {
		*defaultUserCollectionModel
	}
)

func (m customUserCollectionModel) FindAllByUid(ctx context.Context, uid int64) ([]*UserCollection, error) {
	var resp []*UserCollection
	query := fmt.Sprintf("select %s from %s where `uid` = ? and is_delete = 0", userCollectionRows, m.table)
	err := m.QueryRowsNoCacheCtx(ctx, &resp, query, uid)
	switch err {
	case nil:
		return resp, nil
	case sqlc.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}

// NewUserCollectionModel returns a model for the database table.
func NewUserCollectionModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) UserCollectionModel {
	return &customUserCollectionModel{
		defaultUserCollectionModel: newUserCollectionModel(conn, c, opts...),
	}
}

func (m customUserCollectionModel) UpdateIsDelete(ctx context.Context, data *UserCollection) error {
	userCollectionIdKey := fmt.Sprintf("%s%v", cacheUserCollectionIdPrefix, data.Id)
	//cacheUserCollectionIdPrefix：是预定义的缓存键前缀（如 cache:usercollection:id:），用于区分不同类型的缓存
	_, err := m.ExecCtx(ctx, func(ctx context.Context, conn sqlx.SqlConn) (result sql.Result, err error) {
		// 定义更新 SQL：将指定 id 的记录的 is_delete 设为 1
		query := fmt.Sprintf("update %s set is_delete = 1 where `id` = ?", m.table)
		// 执行 SQL，参数为 data.Id（要更新的记录 id）
		return conn.ExecCtx(ctx, query, data.Id)
	}, userCollectionIdKey)
	return err
}
