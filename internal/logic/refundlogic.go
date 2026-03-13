package logic

import (
	"context"

	"github.com/wansui976/go_zero_shop/internal/svc"
	"github.com/wansui976/go_zero_shop/pay"

	"github.com/zeromicro/go-zero/core/logx"
)

type RefundLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRefundLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RefundLogic {
	return &RefundLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 发起退款
func (l *RefundLogic) Refund(in *pay.RefundRequest) (*pay.RefundResponse, error) {
	// todo: add your logic here and delete this line

	return &pay.RefundResponse{}, nil
}
