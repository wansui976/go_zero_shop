package logic

import (
	"context"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/wansui976/go_zero_shop/apps/pay/rpc/internal/svc"
	"github.com/wansui976/go_zero_shop/apps/pay/rpc/pay"
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
	// 1. 查询支付单
	payment, err := l.svcCtx.PaymentModel.FindOneByPaymentId(l.ctx, in.PaymentId)
	if err != nil {
		logx.Errorf("PaymentCallback: payment not found, paymentId: %s, err: %v", in.PaymentId, err)
		return &pay.PaymentCallbackResponse{
			Success: false,
			Message: "payment not found",
		}, nil
	}

	// 2. 验签（实际需要验证第三方签名）
	// if !verifySign(in) {
	//     return &pay.PaymentCallbackResponse{
	//         Success: false,
	//         Message: "sign verify failed",
	//     }, nil
	// }

	// 3. 更新支付状态
	payment.Status = int(in.Status)
	payment.TransactionId = in.TransactionId
	payment.PayTime = time.Now().Unix()

	err = l.svcCtx.PaymentModel.Update(l.ctx, payment)
	if err != nil {
		logx.Errorf("PaymentCallback: update payment failed, err: %v", err)
		return &pay.PaymentCallbackResponse{
			Success: false,
			Message: "update payment failed",
		}, nil
	}

	// 4. 通知订单服务支付成功（可选：调用订单RPC更新状态）
	// _, err = l.svcCtx.OrderRPC.UpdateOrderStatus(...)
	
	return &pay.PaymentCallbackResponse{
		Success: true,
		Message: "success",
	}, nil
}
