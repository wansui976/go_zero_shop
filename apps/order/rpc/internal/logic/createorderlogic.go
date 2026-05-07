package logic

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/wansui976/go_zero_shop/apps/order/rpc/internal/svc"
	"github.com/wansui976/go_zero_shop/apps/order/rpc/model"
	"github.com/wansui976/go_zero_shop/apps/order/rpc/order"
	"github.com/wansui976/go_zero_shop/apps/product/rpc/product"
	"github.com/wansui976/go_zero_shop/apps/user/rpc/user"
	"github.com/wansui976/go_zero_shop/pkg/snowflake"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/mr"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type CreateOrderLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateOrderLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateOrderLogic {
	return &CreateOrderLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CreateOrderLogic) CreateOrder(in *order.CreateOrderRequest) (*order.CreateOrderResponse, error) {
	if in.UserId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "用户ID无效")
	}
	if len(in.Items) == 0 {
		return nil, status.Error(codes.InvalidArgument, "订单必须至少包含一个商品")
	}

	if in.UseDtm {
		return l.createViaDTM(in)
	}
	return l.createDirect(in)
}

// createViaDTM generates a GID and delegates to the DTM-compatible order creation.
func (l *CreateOrderLogic) createViaDTM(in *order.CreateOrderRequest) (*order.CreateOrderResponse, error) {
	orderId, err := snowflake.GenIDInt()
	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("生成订单ID失败: %v", err))
	}
	gid := fmt.Sprintf("order_saga_%d", orderId)

	addReq := &order.AddOrderRequest{
		UserId:      in.UserId,
		Items:       in.Items,
		AddressId:   in.AddressId,
		Gid:         gid,
		PaymentType: 0,
	}

	resp, err := NewCreateOrderDTMLogic(l.ctx, l.svcCtx).CreateOrderDTM(addReq)
	if err != nil {
		return nil, err
	}
	return &order.CreateOrderResponse{
		OrderId: resp.OrderId,
		Success: true,
	}, nil
}

// createDirect performs a single-service order creation without distributed transaction.
func (l *CreateOrderLogic) createDirect(in *order.CreateOrderRequest) (*order.CreateOrderResponse, error) {
	var receiveAddressRes *user.UserReceiveAddress
	productRpcMap := sync.Map{}

	checkUser := func() error {
		userReq := user.UserInfoRequest{Id: in.UserId}
		res, err := l.svcCtx.UserRpc.UserInfo(l.ctx, &userReq)
		if err != nil {
			return fmt.Errorf("检查用户失败: %w", err)
		}
		if res == nil {
			return status.Error(codes.Aborted, fmt.Sprintf("用户不存在(id:%d)", in.UserId))
		}
		return nil
	}

	checkAddress := func() error {
		req := user.UserReceiveAddressInfoRequest{Id: in.AddressId}
		res, err := l.svcCtx.UserRpc.GetUserReceiveAddressInfo(l.ctx, &req)
		if err != nil {
			return fmt.Errorf("检查收货地址失败: %w", err)
		}
		if res == nil {
			return status.Error(codes.Aborted, fmt.Sprintf("收货地址不存在(AddressId:%d)", in.AddressId))
		}
		if res.Uid != in.UserId {
			return status.Error(codes.PermissionDenied, "收货地址不属于当前用户")
		}
		receiveAddressRes = res
		return nil
	}

	checkProducts := func() error {
		for _, item := range in.Items {
			it := item
			res, err := l.svcCtx.ProductRpc.Product(l.ctx, &product.ProductItemRequest{
				ProductId: it.ProductId,
			})
			if err != nil {
				return fmt.Errorf("商品查询失败(id:%d): %w", it.ProductId, err)
			}
			if res == nil {
				return fmt.Errorf("商品不存在(id:%d)", it.ProductId)
			}
			if res.Stock < it.Quantity {
				return fmt.Errorf("库存不足(id:%d need:%d stock:%d)",
					it.ProductId, it.Quantity, res.Stock)
			}
			productRpcMap.Store(it.ProductId, res)
		}
		return nil
	}

	if err := mr.Finish(checkUser, checkAddress, checkProducts); err != nil {
		return nil, err
	}

	orderId, err := snowflake.GenIDInt()
	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("生成订单ID失败: %v", err))
	}
	orderIDText := strconv.FormatInt(orderId, 10)

	db := l.svcCtx.DB
	tx, err := db.BeginTx(l.ctx, nil)
	if err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("开启事务失败: %v", err))
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	nowMs := time.Now().UnixMilli()

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
	if _, err := l.svcCtx.ShippingModel.TxInsert(l.ctx, tx, &shipping); err != nil {
		err = fmt.Errorf("物流插入失败: %w", err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	var orderTotalPrice int64
	for _, it := range in.Items {
		prodRaw, ok := productRpcMap.Load(it.ProductId)
		if !ok {
			err = fmt.Errorf("商品信息缺失(id:%d)", it.ProductId)
			return nil, status.Error(codes.Internal, err.Error())
		}
		prod, ok := prodRaw.(*product.ProductItem)
		if !ok || prod == nil {
			err = fmt.Errorf("商品信息解析失败(id:%d)", it.ProductId)
			return nil, status.Error(codes.Internal, err.Error())
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
			err = fmt.Errorf("插入订单项失败(id:%d): %w", it.ProductId, err)
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	orderModel := model.Orders{
		Id:         orderId,
		OrderId:    orderIDText,
		UserId:     in.UserId,
		TotalPrice: orderTotalPrice,
		Status:     1,
		ReceiverName:    receiveAddressRes.Name,
		ReceiverPhone:   receiveAddressRes.Phone,
		ReceiverAddress: fmt.Sprintf("%s%s%s %s",
			receiveAddressRes.Province, receiveAddressRes.City, receiveAddressRes.Region, receiveAddressRes.DetailAddress),
		CreateTime: nowMs,
		UpdateTime: nowMs,
	}

	if err := l.svcCtx.OrderModel.TxInsert(l.ctx, tx, &orderModel); err != nil {
		err = fmt.Errorf("插入订单主表失败: %w", err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	if err := tx.Commit(); err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("提交事务失败: %v", err))
	}

	event := map[string]interface{}{
		"event":    "OrderCreated",
		"order_id": orderIDText,
		"user_id":  in.UserId,
		"time":     time.Now(),
		"items":    in.Items,
	}
	msg, _ := json.Marshal(event)

	if l.svcCtx.RabbitMQ != nil {
		_ = l.svcCtx.RabbitMQ.Publish("", "order_create_queue", false, false,
			amqp.Publishing{
				ContentType:  "application/json",
				Body:         msg,
				DeliveryMode: amqp.Persistent,
			})
	}

	return &order.CreateOrderResponse{
		OrderId: orderIDText,
		Success: true,
	}, nil
}

