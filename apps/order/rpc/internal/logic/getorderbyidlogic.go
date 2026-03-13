package logic

import (
	"context"
	"github.com/wansui976/go_zero_shop/apps/order/rpc/internal/svc"
	"github.com/wansui976/go_zero_shop/apps/order/rpc/order"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetOrderByIdLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetOrderByIdLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetOrderByIdLogic {
	return &GetOrderByIdLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 查询订单详情
func (l *GetOrderByIdLogic) GetOrderById(in *order.GetOrderByIdRequest) (*order.GetOrderByIdResponse, error) {
	// 1. 根据ID查询订单主表信息
	ordersInfo, err := l.svcCtx.OrderModel.FindOne(l.ctx, in.Id)
	if err != nil {
		return nil, err
	}

	// 2. 查询订单商品列表
	orderItems, err := l.queryOrderItemsByOrderId(l.ctx, ordersInfo.OrderId)
	if err != nil {
		return nil, err
	}

	// 3. 构建响应数据
	return &order.GetOrderByIdResponse{
		Order: &order.OrderDetail{
			Base: &order.OrderBase{
				Id:         ordersInfo.Id,
				OrderId:    ordersInfo.OrderId,
				UserId:     ordersInfo.UserId,
				Status:     order.OrderStatus(ordersInfo.Status),
				CreateTime: ordersInfo.CreateTime,
				UpdateTime: ordersInfo.UpdateTime,
			},
			Products: orderItems,
			Receiver: &order.ReceiverInfo{
				ReceiverName:    ordersInfo.ReceiverName,
				ReceiverPhone:   ordersInfo.ReceiverPhone,
				ReceiverAddress: ordersInfo.ReceiverAddress,
			},
			PaymentAmount: ordersInfo.TotalPrice,
			PaymentType:   order.PaymentType(ordersInfo.PaymentType),
		},
	}, nil
}

// 根据订单ID查询订单商品列表
func (l *GetOrderByIdLogic) queryOrderItemsByOrderId(ctx context.Context, orderId string) ([]*order.OrderProductItem, error) {
	// 使用事务查询订单商品列表
	tx, err := l.svcCtx.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
			return
		}
		tx.Commit()
	}()

	orderItems, err := l.svcCtx.OrderitemModel.TxListByOrderId(ctx, tx, orderId)
	if err != nil {
		return nil, err
	}

	var result []*order.OrderProductItem
	for _, item := range orderItems {
		result = append(result, &order.OrderProductItem{
			ProductId: item.ProductId,
			Quantity:  item.Quantity,
		})
	}

	return result, nil
}
