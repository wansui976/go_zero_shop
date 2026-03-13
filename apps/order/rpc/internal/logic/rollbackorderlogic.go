package logic

import (
	"context"

	"github.com/wansui976/go_zero_shop/apps/order/rpc/internal/svc"
	"github.com/wansui976/go_zero_shop/apps/order/rpc/order"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type RollbackOrderLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRollbackOrderLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RollbackOrderLogic {
	return &RollbackOrderLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 订单回滚（普通下单失败兜底）
func (l *RollbackOrderLogic) RollbackOrder(in *order.CreateOrderRequest) (*order.CreateOrderResponse, error) {
	return nil, status.Error(codes.Unimplemented, "RollbackOrder 尚未实现，请改用 CreateOrderDTMRevert 接口")
}
