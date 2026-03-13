package logic

import (
	"context"

	"github.com/wansui976/go_zero_shop/apps/order/rpc/internal/svc"
	"github.com/wansui976/go_zero_shop/apps/order/rpc/order"

	"github.com/zeromicro/go-zero/core/logx"
)

type OrdersLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewOrdersLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OrdersLogic {
	return &OrdersLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 查询订单列表
func (l *OrdersLogic) Orders(in *order.OrdersRequest) (*order.OrdersResponse, error) {
	// 调用模型层查询订单列表（不分页）
	orders, err := l.svcCtx.OrderModel.FindListByUserId(
		l.ctx,
		in.UserId,
		int64(in.Status),
	)
	if err != nil {
		l.Logger.Errorf("查询订单列表失败: %v", err)
		return nil, err
	}

	// 3. 构建响应
	resp := &order.OrdersResponse{}
	if len(orders) > 0 {
		resp.OrderIds = make([]int64, 0, len(orders))
		for _, order := range orders {
			resp.OrderIds = append(resp.OrderIds, order.Id)
		}
	}

	return resp, nil
}
