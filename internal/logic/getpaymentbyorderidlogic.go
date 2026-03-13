package logic

import (
	"context"

	"github.com/wansui976/go_zero_shop/internal/svc"
	"github.com/wansui976/go_zero_shop/pay"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetPaymentByOrderIdLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetPaymentByOrderIdLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetPaymentByOrderIdLogic {
	return &GetPaymentByOrderIdLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 查询支付状态
func (l *GetPaymentByOrderIdLogic) GetPaymentByOrderId(in *pay.GetPaymentByOrderIdRequest) (*pay.GetPaymentByOrderIdResponse, error) {
	// todo: add your logic here and delete this line

	return &pay.GetPaymentByOrderIdResponse{}, nil
}
