package logic

import (
	"context"

	"github.com/wansui976/go_zero_shop/internal/svc"
	"github.com/wansui976/go_zero_shop/pay"

	"github.com/zeromicro/go-zero/core/logx"
)

type PaymentCallbackLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewPaymentCallbackLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PaymentCallbackLogic {
	return &PaymentCallbackLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 支付回调（第三方支付回调）
func (l *PaymentCallbackLogic) PaymentCallback(in *pay.PaymentCallbackRequest) (*pay.PaymentCallbackResponse, error) {
	// todo: add your logic here and delete this line

	return &pay.PaymentCallbackResponse{}, nil
}
