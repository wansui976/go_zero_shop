package logic

import (
	"context"
	"fmt"

	"github.com/wansui976/go_zero_shop/apps/order/rpc/internal/svc"
	"github.com/wansui976/go_zero_shop/apps/order/rpc/order"
	"github.com/zeromicro/go-zero/core/logx"
)

type CancelOrderLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCancelOrderLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CancelOrderLogic {
	return &CancelOrderLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CancelOrderLogic) CancelOrder(in *order.CancelOrderRequest) (*order.CancelOrderResponse, error) {
	if in.Id <= 0 {
		return &order.CancelOrderResponse{Success: false, Message: "invalid order id"}, fmt.Errorf("invalid order id")
	}

	logic := NewUpdateStatusLogic(l.ctx, l.svcCtx)
	if err := logic.UpdateOrderStatus(in.Id, 0, in.Operator, in.Reason); err != nil {
		return &order.CancelOrderResponse{Success: false, Message: err.Error()}, err
	}

	return &order.CancelOrderResponse{Success: true, Message: "success"}, nil
}
