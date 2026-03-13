package logic

import (
	"context"
	"fmt"
	"time"

	"github.com/wansui976/go_zero_shop/apps/pay/rpc/internal/svc"
	"github.com/wansui976/go_zero_shop/apps/pay/rpc/pay"
	"github.com/zeromicro/go-zero/core/logx"
)

type PaymentCallbackLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewPaymentCallbackLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PaymentCallbackLogic {
	return &PaymentCallbackLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// PaymentCallback 处理第三方支付回调
func (l *PaymentCallbackLogic) PaymentCallback(in *pay.PaymentCallbackRequest) (*pay.PaymentCallbackResponse, error) {
	payment, err := l.svcCtx.PaymentModel.FindOneByPaymentId(l.ctx, in.PaymentId)
	if err != nil {
		logx.Errorf("PaymentCallback: payment not found, paymentId: %s, err: %v", in.PaymentId, err)
		return &pay.PaymentCallbackResponse{Success: false, Message: "payment not found"}, nil
	}

	// 幂等：已完成支付且第三方流水号一致时直接返回成功。
	if payment.Status == int(pay.PaymentStatus_PaymentStatusPaid) && payment.TransactionId == in.TransactionId {
		return &pay.PaymentCallbackResponse{Success: true, Message: "success"}, nil
	}

	payment.Status = int(in.Status)
	payment.TransactionId = in.TransactionId
	payment.PayTime = time.Now().Unix()

	if err = l.svcCtx.PaymentModel.Update(l.ctx, payment); err != nil {
		logx.Errorf("PaymentCallback: update payment failed, err: %v", err)
		return &pay.PaymentCallbackResponse{Success: false, Message: "update payment failed"}, nil
	}

	if in.Status == pay.PaymentStatus_PaymentStatusPaid {
		if err := l.markOrderPaid(payment.OrderId); err != nil {
			logx.Errorf("PaymentCallback: mark order paid failed, orderId: %s, err: %v", payment.OrderId, err)
			return &pay.PaymentCallbackResponse{Success: false, Message: "mark order paid failed"}, nil
		}
	}

	return &pay.PaymentCallbackResponse{Success: true, Message: "success"}, nil
}

func (l *PaymentCallbackLogic) markOrderPaid(orderID string) error {
	ord, err := l.svcCtx.OrderModel.FindOneByOrderId(l.ctx, orderID)
	if err != nil {
		return err
	}
	if ord == nil {
		return nil
	}
	if ord.Status != 1 {
		return nil
	}
	if _, err := l.svcCtx.OrderModel.UpdateStatusIf(l.ctx, ord.Id, 1, 2); err != nil {
		return err
	}
	if l.svcCtx.BizRdb != nil {
		_ = l.svcCtx.BizRdb.Set(l.ctx, fmt.Sprintf("order:paid:%d", ord.Id), "1", 7*24*time.Hour).Err()
	}
	return nil
}
