package order

import (
	"context"

	"github.com/wansui976/go_zero_shop/apps/app/api/internal/middleware"
	"github.com/wansui976/go_zero_shop/apps/app/api/internal/svc"
	"github.com/wansui976/go_zero_shop/apps/app/api/internal/types"
	"github.com/wansui976/go_zero_shop/apps/order/rpc/order"

	"github.com/zeromicro/go-zero/core/logx"
)

type OrderListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取订单列表
func NewOrderListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OrderListLogic {
	return &OrderListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *OrderListLogic) OrderList(req *types.OrderListRequest) (resp *types.CommonResponse, err error) {
	// 从上下文中获取用户ID
	uid, err := middleware.GetUserIDFromCtx(l.ctx)
	if err != nil {
		l.Errorf("获取用户ID失败, err: %v", err)
		return &types.CommonResponse{
			ResultCode: 401,
			Msg:        "未授权",
		}, err
	}

	// 调用order RPC获取订单ID列表
	orderResp, err := l.svcCtx.OrderRPC.Orders(l.ctx, &order.OrdersRequest{
		UserId: uid,
		Status: 0, // 暂时不支持按状态过滤
	})
	if err != nil {
		l.Errorf("获取订单ID列表失败, err: %v", err)
		return &types.CommonResponse{
			ResultCode: 500,
			Msg:        "获取订单列表失败",
		}, err
	}

	// 遍历订单ID，调用GetOrderById获取每个订单的详细信息
	var orders []*types.Order
	for _, orderId := range orderResp.OrderIds {
		orderDetailResp, err := l.svcCtx.OrderRPC.GetOrderById(l.ctx, &order.GetOrderByIdRequest{
			Id: orderId,
		})
		if err != nil {
			l.Errorf("获取订单详情失败, orderId: %d, err: %v", orderId, err)
			continue // 继续获取其他订单，避免因单个订单失败影响整体
		}

		// 将RPC返回的OrderDetail转换为API层的Order类型
		detail := orderDetailResp.Order
		apiOrder := &types.Order{
			OrderID:         detail.Base.OrderId,
			Status:          int32(detail.Base.Status),
			Payment:         detail.PaymentAmount,
			Payment_type:    int32(detail.PaymentType),
			TotalPrice:      detail.PaymentAmount, // 简化处理，实际应从订单明细计算
			CreateTime:      detail.Base.CreateTime,
			ProductItems:    make([]*types.OrderProductItem, 0),
			ReceiverName:    detail.Receiver.ReceiverName,
			ReceiverPhone:   detail.Receiver.ReceiverPhone,
			ReceiverAddress: detail.Receiver.ReceiverAddress,
		}

		// 添加商品详情
		for _, item := range detail.Products {
			apiOrder.ProductItems = append(apiOrder.ProductItems, &types.OrderProductItem{
				ProductID: item.ProductId,
				Quantity:  item.Quantity,
			})
		}

		orders = append(orders, apiOrder)
	}

	// 构建响应
	return &types.CommonResponse{
		ResultCode: 200,
		Msg:        "success",
		Data:       orders,
	}, nil
}
