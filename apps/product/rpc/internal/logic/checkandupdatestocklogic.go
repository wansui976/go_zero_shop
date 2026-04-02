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

const luaCheckAndUpdateScript = `
local stockKey = KEYS[1]
local  requestId = ARGV[1] --幂等性标识:每个请求唯一ID

-- 新增：检查商品是否不存在
local isInvalid = redis.call("HGET", stockKey, "is_invalid")
if isInvalid == "1" then
	return -2 -- 商品不存在，新增返回码
end

--检查幂等性(避免重复扣减)
local isProcessed = redis.call("SISMEMBER",stockKey .. ":processed",requestId)
if isProcessed == 1 then
	return 2 --请求已处理(幂等命中)
end

--检查库存数据结构完整性
local counts = redis.call("HMGET",stockKey,"total","used","frozen")
local total = tonumber(counts[1])
local used = tonumber(counts[2])
local frozen = tonumber(counts[3])

if total == nil or used == nil or frozen == nil then
	return -1
end

--计算可用库存
local available = total - used - frozen
if available < 1 then
	return 0 --库存不足
end

--扣减库存，记录幂等标识
redis.call("HINCRBY",stockKey,"used",1)
redis.call("SADD",stockKey .. ":processed",requestId)
--幂等标识设置过期时间	
redis.call("EXPIRE",stockKey .. ":processed",7*24*3600)

return 1 --扣减成功

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

	// 1. 构造Redis键
	stockKey := fmt.Sprintf("product:stock:%d", in.ProductId)
	l.Infof("check stock:ProductId=%d,requestId=%s,stocstockKey=%s", in.ProductId, in.RequestId, stockKey)

	//执行 Lua脚本
	val, err := l.svcCtx.BizRedis.EvalCtx(
		l.ctx,
		luaCheckAndUpdateScript,
		[]string{stockKey},
		in.RequestId,
	)
	if err != nil {
		// 完善Redis异常日志（包含上下文信息）
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
	//从数据库查询最新库存
	productInfo, err := l.svcCtx.ProductModel.FindOne(l.ctx, productId)
	if err != nil {
		//处理商品不存在场景（缓存穿透防护）
		if errors.Is(err, model.ErrNotFound) {
			stockKey := fmt.Sprintf("product:stock:%d", productId)
			// 写入空库存标记，设置短过期（如5分钟），避免DB压力
			if err := l.svcCtx.BizRedis.HmsetCtx(l.ctx, stockKey, map[string]string{
				"total":      "0",
				"used":       "0",
				"frozen":     "0",
				"sync_used":  "0",
				"is_invalid": "1", // 标记商品不存在
			}); err != nil {
				return fmt.Errorf("hmset invalid product stock failed: %w", err)
			}
			l.svcCtx.BizRedis.ExpireCtx(l.ctx, stockKey, 5*60)
			return fmt.Errorf("product not Found:%d", productId)
		}
		return fmt.Errorf("query product from db failed: %w", err)
	}

	//同步到Redis，哈希结构：total=总库存，used=已扣减库存，frozen=冻结库存）
	stockKey := fmt.Sprintf("product:stock:%d", productId)
	stockData := map[string]string{
		"total":      strconv.FormatInt(productInfo.Stock, 10), // 总库存
		"used":       "0",                                      // 初始已扣减库存
		"frozen":     "0",                                      // 初始冻结库存
		"sync_used":  "0",                                      // 已同步到DB的used值，初始0
		"is_invalid": "0",                                      // 标记商品有效
	}

	// 3. 写入Redis（使用正确的HmsetCtx参数格式）
	if err := l.svcCtx.BizRedis.HmsetCtx(l.ctx, stockKey, stockData); err != nil {
		return fmt.Errorf("hmset redis failed: %w", err)
	}

	//设置 Redis 库存过期时间
	l.svcCtx.BizRedis.ExpireCtx(l.ctx, stockKey, int(7*24*time.Hour))
	return nil
}

// 异步将redis库同步到数据库
func (l *CheckAndUpdateStockLogic) asyncSyncStockToDB(productId int64) error {
	defer func() {
		if r := recover(); r != nil {
			l.Errorf("async sync stock to db panic: productId=%d, panic=%v", productId, r)
		}
	}()

	// 1. 从 Redis 获取当前已扣减库存
	stockKey := fmt.Sprintf("product:stock:%d", productId)
	fields, err := l.svcCtx.BizRedis.HmgetCtx(l.ctx, stockKey, "used", "sync_used")
	if err != nil {
		errMsg := fmt.Errorf("hget used/sync_used from redis failed: productId=%d, err=%v", productId, err)
		l.Errorf(errMsg.Error())
		return errMsg
	}
	// 解析Redis字段（处理nil值，默认0）
	usedStr := fields[0]
	if usedStr == "" {
		usedStr = "0"
	}
	syncUsedStr := fields[1]
	if syncUsedStr == "" {
		syncUsedStr = "0"
	}
	used, _ := strconv.ParseInt(usedStr, 10, 64)
	syncUsed, _ := strconv.ParseInt(syncUsedStr, 10, 64)

	increment := used - syncUsed
	if increment <= 0 {
		l.Infof("no stock to sync: productId=%d, used=%d, syncUsed=%d", productId, used, syncUsed)
		return nil // 无增量，无需同步
	}

	// 2. 开启数据库事务
	tx := l.svcCtx.Orm.BeginTx(l.ctx, nil)
	if tx.Error != nil {
		errMsg := fmt.Errorf("begin tx failed: productId=%d, err=%v", productId, tx.Error)
		l.Errorf(errMsg.Error())
		return errMsg
	}

	// 事务回滚 defer（确保异常时回滚）
	defer func() {
		if r := recover(); r != nil {
			if rollbackErr := tx.Rollback().Error; rollbackErr != nil {
				l.Errorf("tx rollback failed after panic: productId=%d, err=%v", productId, rollbackErr)
			}
		}
	}()

	result := tx.Exec(
		"UPDATE product SET stock = stock - ?, updated_at = NOW() WHERE id = ? AND stock >= ?",
		increment, // 仅同步增量部分
		productId,
		increment,
	)
	if result.Error != nil {
		if rollbackErr := tx.Rollback().Error; rollbackErr != nil {
			l.Errorf("tx rollback failed after exec error: productId=%d, err=%v", productId, rollbackErr)
		}
		errMsg := fmt.Errorf("update stock failed: productId=%d, err=%v", productId, result.Error)
		l.Errorf(errMsg.Error())
		return errMsg
	}

	if result.RowsAffected == 0 {
		_ = tx.Rollback()
		errMsg := fmt.Errorf("product %d insufficient stock (need %d)", productId, increment)
		l.Errorf(errMsg.Error())
		return errMsg
	}

	// 3. 同步成功后，更新Redis中的sync_used（标记已同步增量）
	if err := l.svcCtx.BizRedis.HsetCtx(l.ctx, stockKey, "sync_used", strconv.FormatInt(used, 10)); err != nil {
		l.Errorf("hset sync_used failed: productId=%d, used=%d, err=%v", productId, used, err)
		// 此处不回滚DB，后续重试会重新同步（保证最终一致性）
	}

	// 5. 提交事务
	if err := tx.Commit().Error; err != nil {
		errMsg := fmt.Errorf("commit tx failed: productId=%d, err=%v", productId, err)
		l.Errorf(errMsg.Error())
		return errMsg
	}

	l.Infof("async sync stock to db success: productId=%d, used=%d", productId, used)
	return nil // 新增：函数末尾必须返回 error（此处无错误则返回 nil）
}

// retryCheckAndUpdateStock 重试库存检查（仅在Redis数据同步后重试1次）
func (l *CheckAndUpdateStockLogic) retryCheckAndUpdateStock(in *product.CheckAndUpdateStockRequest) (*product.CheckAndUpdateStockResponse, error) {
	// 短暂延迟（避免数据库同步未完成）
	time.Sleep(100 * time.Millisecond)
	// 重新执行Lua脚本
	val, err := l.svcCtx.BizRedis.EvalCtx(
		l.ctx,
		luaCheckAndUpdateScript,
		[]string{fmt.Sprintf("product:stock:%d", in.ProductId)},
		in.RequestId,
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
