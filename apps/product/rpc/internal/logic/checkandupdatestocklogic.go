package logic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/hibiken/asynq"
	"github.com/wansui976/go_zero_shop/apps/product/rpc/internal/model"
	"github.com/wansui976/go_zero_shop/apps/product/rpc/internal/svc"
	"github.com/wansui976/go_zero_shop/apps/product/rpc/product"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

type CheckAndUpdateStockLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
	BizRedis *redis.Redis
}

type SyncStockToDBTask struct {
	ProductId int64 `json:"product_id"`
}

func NewCheckAndUpdateStockLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CheckAndUpdateStockLogic {
	return &CheckAndUpdateStockLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 统一 schema：total / available / pre_locked / used / sync_used / is_invalid
// 直购扣减按 Num 件，幂等 by requestId
const luaCheckAndUpdateScript = `
local stockKey = KEYS[1]
local requestId = ARGV[1]
local num = tonumber(ARGV[2])
if num == nil or num < 1 then
	return -3 -- 非法数量
end

local isInvalid = redis.call("HGET", stockKey, "is_invalid")
if isInvalid == "1" then
	return -2 -- 商品不存在
end

local isProcessed = redis.call("SISMEMBER", stockKey .. ":processed", requestId)
if isProcessed == 1 then
	return 2 -- 幂等命中
end

local available = redis.call("HGET", stockKey, "available")
if available == false or available == nil then
	return -1 -- 需触发 DB 同步
end
available = tonumber(available)
if available < num then
	return 0 -- 库存不足
end

redis.call("HINCRBY", stockKey, "available", -num)
redis.call("HINCRBY", stockKey, "used", num)
redis.call("SADD", stockKey .. ":processed", requestId)
redis.call("EXPIRE", stockKey .. ":processed", 7*24*3600)
return 1
`

func (l *CheckAndUpdateStockLogic) CheckAndUpdateStock(in *product.CheckAndUpdateStockRequest) (*product.CheckAndUpdateStockResponse, error) {
	if in.ProductId == 0 {
		l.Errorf("ProductId is required")
		return nil, status.Error(codes.InvalidArgument, "商品 ID 不能为空")
	}

	if in.RequestId == "" {
		l.Errorf("requestId is required for idempotency")
		return nil, status.Error(codes.InvalidArgument, "请求ID不能为空,用于防重复提交")
	}

	num := in.Num
	if num <= 0 {
		num = 1
	}

	// 1. 构造Redis键
	stockKey := fmt.Sprintf("product:stock:%d", in.ProductId)
	l.Infof("check stock:ProductId=%d,requestId=%s,num=%d,stockKey=%s", in.ProductId, in.RequestId, num, stockKey)

	//执行 Lua脚本
	val, err := l.svcCtx.BizRedis.EvalCtx(
		l.ctx,
		luaCheckAndUpdateScript,
		[]string{stockKey},
		in.RequestId,
		num,
	)
	if err != nil {
		// 优化3：完善Redis异常日志（包含上下文信息）
		l.Errorf("redis eval script failed: productId=%d, requestId=%s, err=%v", in.ProductId, in.RequestId, err)
		return nil, status.Error(codes.Internal, "库存服务暂时不可用，请稍后重试")
	}

	//解析 Lua脚本返回值
	scriptResult, ok := val.(int64)
	if !ok {
		l.Errorf("invalid script result type: productId=%d, requestId=%s, result=%v", in.ProductId, in.RequestId, val)
		return nil, status.Error(codes.Internal, "库存检查结果解析失败")
	}

	switch scriptResult {
	case 0:
		return nil, status.Errorf(codes.ResourceExhausted, "商品%d库存不足", in.ProductId)

	case -2:
		return nil, status.Errorf(codes.ResourceExhausted, "商品%d不存在", in.ProductId)

	case -3:
		return nil, status.Error(codes.InvalidArgument, "扣减数量必须大于0")

	case 2:
		return &product.CheckAndUpdateStockResponse{Success: true}, nil // 幂等命中视为成功

	case -1:
		//Redis数据异常（需同步数据库库存到Redis）
		if err := l.syncStockFromDBToRedis(in.ProductId); err != nil {
			l.Errorf("sync stock from db to redis failed: productId=%d, err=%v", in.ProductId, err)
			return nil, status.Error(codes.Internal, "库存数据同步中，请稍后重试")
		}
		// 同步后重试一次（避免用户重复操作）
		return l.retryCheckAndUpdateStock(in)

	case 1:
		// 替换go协程为队列任务
		taskPayload, _ := json.Marshal(SyncStockToDBTask{ProductId: in.ProductId})
		_, err := l.svcCtx.AsynqClient.Enqueue(asynq.NewTask("sync_stock_to_db", taskPayload),
			asynq.MaxRetry(3),             // 最多重试3次
			asynq.Retention(24*time.Hour), // 任务保留24小时，便于排查
		)
		if err != nil {
			l.Errorf("enqueue sync task failed: productId=%d, err=%v", in.ProductId, err)
			// 此处可降级为go调用，避免任务丢失
			go l.asyncSyncStockToDB(in.ProductId)
		}
		return &product.CheckAndUpdateStockResponse{Success: true}, nil
	default:
		// 未知结果（异常场景）
		l.Errorf("unknown script result: productId=%d, requestId=%s, result=%d", in.ProductId, in.RequestId, scriptResult)
		return nil, status.Error(codes.Internal, "库存操作异常，请稍后重试")
	}

}

func (l *CheckAndUpdateStockLogic) syncStockFromDBToRedis(productId int64) error {
	productInfo, err := l.svcCtx.ProductModel.FindOne(l.ctx, productId)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			stockKey := fmt.Sprintf("product:stock:%d", productId)
			if err := l.svcCtx.BizRedis.HmsetCtx(l.ctx, stockKey, map[string]string{
				"total":      "0",
				"available":  "0",
				"pre_locked": "0",
				"used":       "0",
				"sync_used":  "0",
				"is_invalid": "1",
			}); err != nil {
				return fmt.Errorf("hmset invalid product stock failed: %w", err)
			}
			_ = l.svcCtx.BizRedis.ExpireCtx(l.ctx, stockKey, 5*60)
			return fmt.Errorf("product not Found:%d", productId)
		}
		return fmt.Errorf("query product from db failed: %w", err)
	}

	stockKey := fmt.Sprintf("product:stock:%d", productId)
	stockData := map[string]string{
		"total":      strconv.FormatInt(productInfo.Stock, 10),
		"available":  strconv.FormatInt(productInfo.Stock, 10),
		"pre_locked": "0",
		"used":       "0",
		"sync_used":  "0",
		"is_invalid": "0",
	}
	if err := l.svcCtx.BizRedis.HmsetCtx(l.ctx, stockKey, stockData); err != nil {
		return fmt.Errorf("hmset redis failed: %w", err)
	}
	_ = l.svcCtx.BizRedis.ExpireCtx(l.ctx, stockKey, int((7 * 24 * time.Hour).Seconds()))
	return nil
}

// 原子读 used/sync_used，避免读取期间被其他写入插入
const luaReadStockUsedScript = `
local used = tonumber(redis.call("HGET", KEYS[1], "used")) or 0
local syncUsed = tonumber(redis.call("HGET", KEYS[1], "sync_used")) or 0
return {used, syncUsed}
`

// CAS 提交 sync_used：仅当当前值等于 expected 才更新到 newVal
const luaCASSyncUsedScript = `
local cur = tonumber(redis.call("HGET", KEYS[1], "sync_used")) or 0
local expected = tonumber(ARGV[1])
local newVal = tonumber(ARGV[2])
if cur ~= expected then
	return 0
end
redis.call("HSET", KEYS[1], "sync_used", newVal)
return 1
`

// 异步将 Redis 增量同步到数据库（用 Lua CAS 避免读改写竞态）
func (l *CheckAndUpdateStockLogic) asyncSyncStockToDB(productId int64) error {
	defer func() {
		if r := recover(); r != nil {
			l.Errorf("async sync stock to db panic: productId=%d, panic=%v", productId, r)
		}
	}()

	stockKey := fmt.Sprintf("product:stock:%d", productId)

	// 1. 原子读 used 与 sync_used
	raw, err := l.svcCtx.BizRedis.EvalCtx(l.ctx, luaReadStockUsedScript, []string{stockKey})
	if err != nil {
		l.Errorf("read used/sync_used via lua failed: productId=%d, err=%v", productId, err)
		return err
	}
	arr, ok := raw.([]interface{})
	if !ok || len(arr) < 2 {
		l.Errorf("invalid lua read result: productId=%d, raw=%v", productId, raw)
		return fmt.Errorf("invalid lua read result")
	}
	used, _ := arr[0].(int64)
	syncUsed, _ := arr[1].(int64)

	increment := used - syncUsed
	if increment <= 0 {
		l.Infof("no stock to sync: productId=%d, used=%d, syncUsed=%d", productId, used, syncUsed)
		return nil
	}

	// 2. DB 事务扣减
	tx := l.svcCtx.Orm.BeginTx(l.ctx, nil)
	if tx.Error != nil {
		return fmt.Errorf("begin tx failed: productId=%d, err=%v", productId, tx.Error)
	}
	defer func() {
		if r := recover(); r != nil {
			_ = tx.Rollback()
		}
	}()

	result := tx.Exec(
		"UPDATE product SET stock = stock - ?, update_time = NOW() WHERE id = ? AND stock >= ?",
		increment, productId, increment,
	)
	if result.Error != nil {
		_ = tx.Rollback()
		return fmt.Errorf("update stock failed: productId=%d, err=%v", productId, result.Error)
	}
	if result.RowsAffected == 0 {
		_ = tx.Rollback()
		return fmt.Errorf("product %d insufficient stock (need %d)", productId, increment)
	}
	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("commit tx failed: productId=%d, err=%v", productId, err)
	}

	// 3. CAS 提交 sync_used；失败说明有并发 worker 已提交，本次的增量已被覆盖统计
	casRaw, err := l.svcCtx.BizRedis.EvalCtx(l.ctx, luaCASSyncUsedScript,
		[]string{stockKey},
		strconv.FormatInt(syncUsed, 10),
		strconv.FormatInt(used, 10),
	)
	if err != nil {
		l.Errorf("cas sync_used failed: productId=%d, err=%v", productId, err)
		return nil // DB 已扣，Redis 同步 CAS 失败由下次定时任务兜底
	}
	if code, _ := casRaw.(int64); code == 0 {
		l.Infof("cas sync_used lost race (other worker won): productId=%d, expected=%d, used=%d", productId, syncUsed, used)
	}

	l.Infof("async sync stock to db success: productId=%d, used=%d", productId, used)
	return nil
}

// retryCheckAndUpdateStock 重试库存检查（仅在Redis数据同步后重试1次）
func (l *CheckAndUpdateStockLogic) retryCheckAndUpdateStock(in *product.CheckAndUpdateStockRequest) (*product.CheckAndUpdateStockResponse, error) {
	num := in.Num
	if num <= 0 {
		num = 1
	}
	time.Sleep(100 * time.Millisecond)
	val, err := l.svcCtx.BizRedis.EvalCtx(
		l.ctx,
		luaCheckAndUpdateScript,
		[]string{fmt.Sprintf("product:stock:%d", in.ProductId)},
		in.RequestId,
		num,
	)
	if err != nil {
		l.Errorf("retry redis eval failed: productId=%d, requestId=%s, err=%v", in.ProductId, in.RequestId, err)
		return nil, status.Error(codes.Internal, "重试库存操作失败，请稍后重试")
	}

	scriptResult, ok := val.(int64)
	if !ok || scriptResult != 1 {
		l.Errorf("retry script result invalid: productId=%d, requestId=%s, result=%v", in.ProductId, in.RequestId, val)
		return nil, status.Error(codes.Internal, "库存同步后操作仍失败，请稍后重试")
	}

	l.Infof("retry stock update success: productId=%d, requestId=%s", in.ProductId, in.RequestId)
	go l.asyncSyncStockToDB(in.ProductId)
	return &product.CheckAndUpdateStockResponse{Success: true}, nil
}
