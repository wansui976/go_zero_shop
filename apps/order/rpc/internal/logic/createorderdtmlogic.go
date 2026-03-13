package logic

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/dtm-labs/dtmgrpc"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/redis/go-redis/v9"
	"github.com/wansui976/go_zero_shop/apps/order/rpc/internal/svc"
	"github.com/wansui976/go_zero_shop/apps/order/rpc/model"
	"github.com/wansui976/go_zero_shop/apps/order/rpc/order"
	"github.com/wansui976/go_zero_shop/apps/pay/rpc/pay"
	"github.com/wansui976/go_zero_shop/apps/pay/rpc/payclient"
	"github.com/wansui976/go_zero_shop/apps/product/rpc/product"
	"github.com/wansui976/go_zero_shop/apps/user/rpc/user"
	"github.com/wansui976/go_zero_shop/pkg/snowflake"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/mr"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type CreateOrderDTMLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateOrderDTMLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateOrderDTMLogic {
	return &CreateOrderDTMLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// Try：创建半成品订单
func (l *CreateOrderDTMLogic) CreateOrderDTM(in *order.AddOrderRequest) (*order.AddOrderResponse, error) {
	if in.Gid == "" {
		return nil, status.Error(codes.InvalidArgument, "全局事务ID不能为空")
	}
	if in.UserId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "用户ID无效")
	}
	if len(in.Items) == 0 {
		return nil, status.Error(codes.InvalidArgument, "订单必须至少包含一个商品")
	}
	var (
		userRpcRes        *user.UserInfoResponse
		receiveAddressRes *user.UserReceiveAddress
	)
	productRpcMap := sync.Map{}

	//并行预检查（用户、地址、多商品库存
	//检查用户
	checkUser := func() error {
		var err error
		var userReq user.UserInfoRequest
		userReq.Id = in.UserId
		userRpcRes, err = l.svcCtx.UserRpc.UserInfo(l.ctx, &userReq)
		if err != nil {
			return fmt.Errorf("检查用户失败:%w", err)
		}
		if userRpcRes == nil {
			return status.Error(codes.Aborted, fmt.Sprintf("用户不存在(id:%d)", in.UserId))
		}
		return nil
	}

	//查询收货地址
	checkUserReceiveAddress := func() error {
		var err error
		var userReceiveAddressInfoReq user.UserReceiveAddressInfoRequest
		userReceiveAddressInfoReq.Id = in.AddressId
		receiveAddressRes, err = l.svcCtx.UserRpc.GetUserReceiveAddressInfo(l.ctx, &userReceiveAddressInfoReq)
		if err != nil {
			return fmt.Errorf("检查收货地址失败	: %w", err)
		}
		if receiveAddressRes == nil {

			return status.Error(codes.Aborted, fmt.Sprintf("收货地址不存在(AddressId:%d)", in.AddressId))
		}
		if receiveAddressRes.Uid != in.UserId {
			return status.Error(codes.PermissionDenied, "收货地址不属于当前用户")
		}
		return nil
	}

	//检查商品
	checkProducts := func() error {
		var eg errgroup.Group
		for _, item := range in.Items {
			it := item
			eg.Go(func() error {
				res, err := l.svcCtx.ProductRpc.Product(l.ctx, &product.ProductItemRequest{
					ProductId: it.ProductId,
				})
				if err != nil {
					return fmt.Errorf("商品查询失败(id:%d):%w", it.ProductId, err)
				}
				if res == nil {
					return fmt.Errorf("商品不存在(id:%d)", it.ProductId)
				}
				if res.Stock < it.Quantity {
					return fmt.Errorf("库存不足(id:%d need:%d stock:%d)",
						it.ProductId, it.Quantity, res.Stock)
				}

				productRpcMap.Store(it.ProductId, res)
				return nil

			})
		}
		return eg.Wait()
	}
	if err := mr.Finish(checkUser, checkUserReceiveAddress, checkProducts); err != nil {
		return nil, err
	}

	//生成 ID
	orderId, err := snowflake.GenIDInt()
	if err != nil {
		logx.Errorf("生成订单ID失败: %v", err)
		return nil, status.Error(codes.Internal, fmt.Sprintf("生成订单 ID 失败: %v", err))
	}
	orderIDText := strconv.FormatInt(orderId, 10)
	db := l.svcCtx.DB
	barrier, err := dtmgrpc.BarrierFromGrpc(l.ctx)

	if err != nil {

		return nil, status.Error(codes.Internal, err.Error())
	}
	// 执行 Try 子事务：DB + Redis 预扣库存。预扣 key 统一基于 gid，便于 Confirm/Revert 幂等处理。
	err = barrier.CallWithDB(db, func(tx *sql.Tx) error {
		var preLockedItems []*order.OrderProductItem
		revertPreLocks := func() {
			if l.svcCtx == nil || l.svcCtx.Rdb == nil {
				return
			}
			for _, it := range preLockedItems {
				_ = revertPreLockStockByGID(l.ctx, l.svcCtx.Rdb, it.ProductId, in.Gid)
			}
		}

		if l.svcCtx != nil && l.svcCtx.Rdb != nil {
			for _, it := range in.Items {
				if err := preLockStockByGID(l.ctx, l.svcCtx.Rdb, it.ProductId, in.Gid, it.Quantity); err != nil {
					revertPreLocks()
					return status.Error(codes.ResourceExhausted, fmt.Sprintf("预扣库存失败(product_id:%d): %v", it.ProductId, err))
				}
				preLockedItems = append(preLockedItems, it)
			}
		}

		nowMs := time.Now().UnixMilli()

		//① 插入物流信息
		shipping := model.OrderAddressSnapshot{
			OrderId:  orderIDText,
			UserId:   in.UserId,
			Name:     receiveAddressRes.Name,
			Phone:    receiveAddressRes.Phone,
			Province: receiveAddressRes.Province,
			City:     receiveAddressRes.City,
			District: receiveAddressRes.Region,
			Detail:   receiveAddressRes.DetailAddress,
		}

		_, err := l.svcCtx.ShippingModel.TxInsert(l.ctx, tx, &shipping)
		if err != nil {
			revertPreLocks()
			return fmt.Errorf("物流插入失败:%w", err)
		}

		//② 插入订单项（多商品）
		var orderTotalPrice int64

		for _, it := range in.Items {
			prodRaw, ok := productRpcMap.Load(it.ProductId)
			if !ok {
				revertPreLocks()
				return fmt.Errorf("商品信息缺失(id:%d)", it.ProductId)
			}
			prod, ok := prodRaw.(*product.ProductItem)
			if !ok || prod == nil {
				revertPreLocks()
				return fmt.Errorf("商品信息解析失败(id:%d)", it.ProductId)
			}

			item := model.OrderItems{
				OrderId:    orderIDText,
				ProductId:  it.ProductId,
				UnitPrice:  prod.Price,
				Quantity:   it.Quantity,
				TotalPrice: prod.Price * it.Quantity,
				CreateTime: nowMs,
				UpdateTime: nowMs,
			}
			orderTotalPrice += item.TotalPrice

			if err := l.svcCtx.OrderitemModel.TxInsert(l.ctx, tx, &item); err != nil {
				revertPreLocks()
				return fmt.Errorf("插入订单项失败:id=%d err=%w", it.ProductId, err)
			}

		}

		// 生成订单唯一编号
		orderModel := model.Orders{
			Id:              orderId,
			OrderId:         orderIDText,
			UserId:          in.UserId,
			TotalPrice:      int64(orderTotalPrice),
			PaymentType:     in.PaymentType,
			Status:          1,
			ReceiverName:    receiveAddressRes.Name,
			ReceiverPhone:   receiveAddressRes.Phone,
			ReceiverAddress: receiveAddressRes.Province + receiveAddressRes.Region + receiveAddressRes.City + receiveAddressRes.DetailAddress,
			Gid: sql.NullString{
				String: in.Gid,
				Valid:  in.Gid != "", // 若Gid非空则Valid为true，否则false（对应数据库NULL）
			},

			CreateTime: nowMs,
			UpdateTime: nowMs,
		}

		if err := l.svcCtx.OrderModel.TxInsert(l.ctx, tx, &orderModel); err != nil {
			revertPreLocks()
			return fmt.Errorf("插入订单主表失败:%w", err)
		}

		return nil // 事务正常结束

	})

	if err != nil {
		if l.svcCtx != nil && l.svcCtx.Rdb != nil {
			for _, it := range in.Items {
				_ = revertPreLockStockByGID(l.ctx, l.svcCtx.Rdb, it.ProductId, in.Gid)
			}
		}
		return nil, err
	}

	if l.svcCtx != nil && l.svcCtx.Rdb != nil {
		_ = l.svcCtx.Rdb.Set(l.ctx, "order:gid_to_order:"+in.Gid, orderIDText, 24*time.Hour).Err()
	}

	// DTM 重试时 Barrier 可能会跳过执行，这里统一以 gid 回查真实 order_id 返回。
	if existingOrderID, qerr := l.getOrderIDByGID(in.Gid); qerr != nil {
		logx.Errorf("按gid查询订单失败(gid:%s): %v", in.Gid, qerr)
	} else if existingOrderID != "" {
		orderIDText = existingOrderID
	}

	if err := l.publishOrderCreatedEvent(orderId, in.UserId, in.Items); err != nil {
		l.Errorf("发布订单创建事件失败(orderId=%d, gid=%s): %v", orderId, in.Gid, err)
	}
	if err := l.publishDelayEvents(orderId, in.UserId, in.Items); err != nil {
		l.Errorf("发布订单延迟事件失败(orderId=%d, gid=%s): %v", orderId, in.Gid, err)
	}
	if err := l.createPayment(orderIDText, in.UserId, orderTotalAmount(in.Items, productRpcMap), in.PaymentType); err != nil {
		l.Errorf("创建支付单失败(orderId=%s, gid=%s): %v", orderIDText, in.Gid, err)
	}

	return &order.AddOrderResponse{OrderId: orderIDText}, nil
}

func orderTotalAmount(items []*order.OrderProductItem, productRpcMap sync.Map) int64 {
	var total int64
	for _, item := range items {
		if raw, ok := productRpcMap.Load(item.ProductId); ok {
			if prod, ok := raw.(*product.ProductItem); ok && prod != nil {
				total += prod.Price * item.Quantity
			}
		}
	}
	return total
}

func (l *CreateOrderDTMLogic) createPayment(orderID string, userID int64, amount int64, paymentType int64) error {
	if l.svcCtx == nil || l.svcCtx.PayRpc == nil || amount <= 0 {
		return nil
	}
	resp, err := l.svcCtx.PayRpc.CreatePayment(l.ctx, &payclient.CreatePaymentReq{
		OrderId:     orderID,
		UserId:      userID,
		Amount:      amount,
		PaymentType: pay.PaymentType(paymentType),
	})
	if err != nil {
		return err
	}
	if l.svcCtx.Rdb != nil && resp != nil {
		payload, mErr := json.Marshal(map[string]interface{}{
			"order_id":    orderID,
			"payment_id":  resp.PaymentId,
			"pay_url":     resp.PayUrl,
			"expire_time": resp.ExpireTime,
		})
		if mErr == nil {
			_ = l.svcCtx.Rdb.Set(l.ctx, "order:payment_info:"+orderID, string(payload), 24*time.Hour).Err()
		}
	}
	return nil
}

func (l *CreateOrderDTMLogic) publishOrderCreatedEvent(orderID int64, userID int64, items []*order.OrderProductItem) error {
	if l.svcCtx == nil || l.svcCtx.RabbitMQ == nil {
		return nil
	}

	type orderCreatedEvent struct {
		Event   string                    `json:"event"`
		OrderID int64                     `json:"order_id"`
		UserID  int64                     `json:"user_id"`
		Time    time.Time                 `json:"time"`
		Items   []*order.OrderProductItem `json:"items"`
	}

	msg, err := json.Marshal(orderCreatedEvent{
		Event:   "OrderCreated",
		OrderID: orderID,
		UserID:  userID,
		Time:    time.Now(),
		Items:   items,
	})
	if err != nil {
		return err
	}

	return l.svcCtx.RabbitMQ.Publish("", "order_create_queue", false, false, amqp.Publishing{
		ContentType:  "application/json",
		Body:         msg,
		DeliveryMode: amqp.Persistent,
	})
}

func (l *CreateOrderDTMLogic) publishDelayEvents(orderID int64, userID int64, items []*order.OrderProductItem) error {
	if l.svcCtx == nil || l.svcCtx.RabbitMQ == nil {
		return nil
	}

	type delayOrderItem struct {
		ProductID int64 `json:"product_id"`
		Quantity  int64 `json:"quantity"`
	}
	type orderDelayMessage struct {
		OrderID   int64            `json:"order_id"`
		UserID    int64            `json:"user_id"`
		Action    string           `json:"action"`
		CreatedAt time.Time        `json:"created_at"`
		Items     []delayOrderItem `json:"items,omitempty"`
	}

	delayItems := make([]delayOrderItem, 0, len(items))
	for _, item := range items {
		delayItems = append(delayItems, delayOrderItem{ProductID: item.ProductId, Quantity: item.Quantity})
	}

	publish := func(action string) error {
		body, err := json.Marshal(orderDelayMessage{
			OrderID:   orderID,
			UserID:    userID,
			Action:    action,
			CreatedAt: time.Now(),
			Items:     delayItems,
		})
		if err != nil {
			return err
		}
		return l.svcCtx.RabbitMQ.Publish("", "order.delay.queue", false, false, amqp.Publishing{
			ContentType:  "application/json",
			Body:         body,
			DeliveryMode: amqp.Persistent,
		})
	}

	if err := publish("remind"); err != nil {
		return err
	}
	return publish("cancel")
}

func (l *CreateOrderDTMLogic) getOrderIDByGID(gid string) (string, error) {
	if gid == "" || l.svcCtx == nil || l.svcCtx.DB == nil {
		return "", nil
	}
	var orderID string
	err := l.svcCtx.DB.QueryRowContext(l.ctx, "SELECT order_id FROM orders WHERE gid = ? LIMIT 1", gid).Scan(&orderID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return orderID, nil
}

// --- Redis Lua scripts and helpers for stock pre-lock/confirm/revert ---

const luaPreLockStock = `
local stockKey = KEYS[1]
local preLockKey = KEYS[2]
local confirmKey = KEYS[3]
local revertKey = KEYS[4]
local quantity = tonumber(ARGV[1])
-- 幂等检查
if redis.call("EXISTS", preLockKey) == 1 then
    return 2
end
if redis.call("EXISTS", confirmKey) == 1 then
    return 3
end
if redis.call("EXISTS", revertKey) == 1 then
    return 4
end
local available = tonumber(redis.call("HGET", stockKey, "available"))
if available == nil or available < quantity then
    return 0
end
redis.call("HINCRBY", stockKey, "available", -quantity)
redis.call("HINCRBY", stockKey, "pre_locked", quantity)
redis.call("SETEX", preLockKey, 86400, quantity)
return 1
`

const luaConfirmStock = `
local stockKey = KEYS[1]
local preLockKey = KEYS[2]
local confirmKey = KEYS[3]
local revertKey = KEYS[4]
if redis.call("EXISTS", confirmKey) == 1 then
    return 2
end
if redis.call("EXISTS", revertKey) == 1 then
    return 3
end
local quantity = tonumber(redis.call("GET", preLockKey))
if quantity == nil then
    return 0
end
redis.call("HINCRBY", stockKey, "pre_locked", -quantity)
redis.call("DEL", preLockKey)
redis.call("SETEX", confirmKey, 86400, quantity)
return 1
`

const luaRevertStock = `
local stockKey = KEYS[1]
local preLockKey = KEYS[2]
local confirmKey = KEYS[3]
local revertKey = KEYS[4]
if redis.call("EXISTS", revertKey) == 1 then
    return 2
end
if redis.call("EXISTS", confirmKey) == 1 then
    return 3
end
local quantity = tonumber(redis.call("GET", preLockKey))
if quantity == nil then
    return 0
end
redis.call("HINCRBY", stockKey, "available", quantity)
redis.call("HINCRBY", stockKey, "pre_locked", -quantity)
redis.call("DEL", preLockKey)
redis.call("SETEX", revertKey, 86400, quantity)
return 1
`

func stockPreLockKey(gid string, productId int64) string {
	return fmt.Sprintf("product:prelock:gid:%s:%d", gid, productId)
}

func stockConfirmKey(gid string, productId int64) string {
	return fmt.Sprintf("product:prelock:gid:%s:%d:confirmed", gid, productId)
}

func stockRevertKey(gid string, productId int64) string {
	return fmt.Sprintf("product:prelock:gid:%s:%d:reverted", gid, productId)
}

func preLockStockByGID(ctx context.Context, rdb *redis.Client, productId int64, gid string, quantity int64) error {
	stockKey := fmt.Sprintf("product:stock:%d", productId)
	preLockKey := stockPreLockKey(gid, productId)
	confirmKey := stockConfirmKey(gid, productId)
	revertKey := stockRevertKey(gid, productId)
	cmd := rdb.Eval(ctx, luaPreLockStock, []string{stockKey, preLockKey, confirmKey, revertKey}, quantity)
	code, err := cmd.Int64()
	if err != nil {
		return fmt.Errorf("redis eval prelock failed: %w", err)
	}
	switch code {
	case 0:
		return errors.New("库存不足")
	case 1:
		return nil
	case 2:
		return nil // 已预扣，幂等
	case 3:
		return errors.New("库存已确认，禁止重复预扣")
	case 4:
		return errors.New("库存已回滚，禁止重复预扣")
	default:
		return errors.New("未知预扣返回码")
	}
}

func confirmPreLockStockByGID(ctx context.Context, rdb *redis.Client, productId int64, gid string) error {
	stockKey := fmt.Sprintf("product:stock:%d", productId)
	preLockKey := stockPreLockKey(gid, productId)
	confirmKey := stockConfirmKey(gid, productId)
	revertKey := stockRevertKey(gid, productId)
	cmd := rdb.Eval(ctx, luaConfirmStock, []string{stockKey, preLockKey, confirmKey, revertKey})
	code, err := cmd.Int64()
	if err != nil {
		return fmt.Errorf("redis eval confirm failed: %w", err)
	}
	switch code {
	case 1, 2:
		return nil
	case 0:
		return errors.New("未找到可确认的预扣库存")
	case 3:
		return errors.New("库存已回滚，无法确认")
	default:
		return errors.New("未知确认返回码")
	}
}

func revertPreLockStockByGID(ctx context.Context, rdb *redis.Client, productId int64, gid string) error {
	stockKey := fmt.Sprintf("product:stock:%d", productId)
	preLockKey := stockPreLockKey(gid, productId)
	confirmKey := stockConfirmKey(gid, productId)
	revertKey := stockRevertKey(gid, productId)
	cmd := rdb.Eval(ctx, luaRevertStock, []string{stockKey, preLockKey, confirmKey, revertKey})
	code, err := cmd.Int64()
	if err != nil {
		return fmt.Errorf("redis eval revert failed: %w", err)
	}
	switch code {
	case 1, 2, 0, 3:
		return nil
	default:
		return errors.New("未知回滚返回码")
	}
}
