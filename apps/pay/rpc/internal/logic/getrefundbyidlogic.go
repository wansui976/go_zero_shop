package logic

import (
	"context"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/wansui976/go_zero_shop/apps/pay/rpc/internal/svc"
	"github.com/wansui976/go_zero_shop/apps/pay/rpc/pay"
)

type GetRefundByIdLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetRefundByIdLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetRefundByIdLogic {
	return &GetRefundByIdLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetRefundByIdLogic) GetRefundById(in *pay.GetRefundByIdRequest) (*pay.GetRefundByIdResponse, error) {
	refund, err := l.svcCtx.RefundModel.FindOneByRefundId(l.ctx, in.RefundId)
	if err != nil {
		logx.Errorf("GetRefundById: refund not found, refundId: %s, err: %v", in.RefundId, err)
		return &pay.GetRefundByIdResponse{
			Refund: nil,
		}, nil
	}

	return &pay.GetRefundByIdResponse{
		Refund: &pay.RefundInfo{
			RefundId:     refund.RefundId,
			OrderId:      refund.OrderId,
			PaymentId:    refund.PaymentId,
			RefundAmount: refund.RefundAmount,
			Status:       pay.RefundStatus(refund.Status),
			Reason:       refund.Reason,
			CreateTime:   refund.CreateTime,
			RefundTime:   refund.RefundTime,
		},
	}, nil
}
