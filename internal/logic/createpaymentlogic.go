package logic

import (
	"context"

	"github.com/wansui976/go_zero_shop/internal/svc"
	"github.com/wansui976/go_zero_shop/pay"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreatePaymentLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreatePaymentLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreatePaymentLogic {
	return &CreatePaymentLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 创建支付订单
func (l *CreatePaymentLogic) CreatePayment(in *pay.CreatePaymentRequest) (*pay.CreatePaymentResponse, error) {
	// todo: add your logic here and delete this line

	return &pay.CreatePaymentResponse{}, nil
}
