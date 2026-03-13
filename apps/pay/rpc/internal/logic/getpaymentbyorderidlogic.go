package logic

import (
	"context"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/wansui976/go_zero_shop/apps/pay/rpc/internal/svc"
	"github.com/wansui976/go_zero_shop/apps/pay/rpc/pay"
)

type GetPaymentByOrderIdLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetPaymentByOrderIdLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetPaymentByOrderIdLogic {
	return &GetPaymentByOrderIdLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetPaymentByOrderIdLogic) GetPaymentByOrderId(in *pay.GetPaymentByOrderIdRequest) (*pay.GetPaymentByOrderIdResponse, error) {
	payment, err := l.svcCtx.PaymentModel.FindOneByOrderId(l.ctx, in.OrderId)
	if err != nil {
		logx.Errorf("GetPaymentByOrderId: payment not found, orderId: %s, err: %v", in.OrderId, err)
		return &pay.GetPaymentByOrderIdResponse{
			Payment: nil,
		}, nil
	}

	return &pay.GetPaymentByOrderIdResponse{
		Payment: &pay.PaymentInfo{
			PaymentId:     payment.PaymentId,
			OrderId:       payment.OrderId,
			UserId:        payment.UserId,
			Amount:        payment.Amount,
			Status:         pay.PaymentStatus(payment.Status),
			PaymentType:   pay.PaymentType(payment.PaymentType),
			CreateTime:    payment.CreateTime,
			PayTime:       payment.PayTime,
			TransactionId: payment.TransactionId,
		},
	}, nil
}
