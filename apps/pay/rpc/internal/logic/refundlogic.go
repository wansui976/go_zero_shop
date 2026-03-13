package logic

import (
	"context"
	"time"

	"github.com/bwmarrin/snowflake"
	"github.com/wansui976/go_zero_shop/apps/pay/rpc/internal/svc"
	"github.com/wansui976/go_zero_shop/apps/pay/rpc/model"
	"github.com/wansui976/go_zero_shop/apps/pay/rpc/pay"
	"github.com/zeromicro/go-zero/core/logx"
)

type RefundLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRefundLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RefundLogic {
	return &RefundLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RefundLogic) Refund(in *pay.RefundRequest) (*pay.RefundResponse, error) {
	if in.RefundAmount <= 0 {
		return &pay.RefundResponse{
			Success: false,
			Message: "invalid refund amount",
		}, nil
	}

	// 1. 查询原支付单
	payment, err := l.svcCtx.PaymentModel.FindOneByPaymentId(l.ctx, in.PaymentId)
	if err != nil {
		logx.Errorf("Refund: payment not found, paymentId: %s, err: %v", in.PaymentId, err)
		return &pay.RefundResponse{
			Success: false,
			Message: "payment not found",
		}, nil
	}

	// 2. 检查支付状态
	if payment.Status != int(pay.PaymentStatus_PaymentStatusPaid) {
		return &pay.RefundResponse{
			Success: false,
			Message: "payment not paid",
		}, nil
	}

	// 3. 校验订单匹配与退款金额
	if payment.OrderId != in.OrderId {
		return &pay.RefundResponse{
			Success: false,
			Message: "order does not match payment",
		}, nil
	}
	if in.RefundAmount > payment.Amount {
		return &pay.RefundResponse{
			Success: false,
			Message: "refund amount exceeds paid amount",
		}, nil
	}
	refundedTotal, err := l.svcCtx.RefundModel.SumRefundAmountByPaymentId(l.ctx, in.PaymentId)
	if err != nil {
		logx.Errorf("Refund: query refund total failed, paymentId: %s, err: %v", in.PaymentId, err)
		return &pay.RefundResponse{
			Success: false,
			Message: "query refund total failed",
		}, nil
	}
	if refundedTotal+in.RefundAmount > payment.Amount {
		return &pay.RefundResponse{
			Success: false,
			Message: "refund amount exceeds remaining payable amount",
		}, nil
	}

	// 4. 创建退款单
	refundId := generateRefundId(l.svcCtx.Config.Snowflake.NodeID)
	_, err = l.svcCtx.RefundModel.Insert(l.ctx, &model.Refund{
		RefundId:     refundId,
		OrderId:      in.OrderId,
		PaymentId:    in.PaymentId,
		RefundAmount: in.RefundAmount,
		Status:       int(pay.RefundStatus_RefundStatusPending),
		Reason:       in.Reason,
		CreateTime:   time.Now().Unix(),
		RefundTime:   0,
	})
	if err != nil {
		logx.Errorf("Refund: insert refund failed, err: %v", err)
		return &pay.RefundResponse{
			Success: false,
			Message: "create refund failed",
		}, nil
	}

	// 5. 调用第三方支付发起退款（实际需要调用微信/支付宝退款API）
	// success := callRefundAPI(payment, refundId, in.RefundAmount)
	success := true // 模拟成功

	if success {
		// 更新退款单状态
		refund, _ := l.svcCtx.RefundModel.FindOneByRefundId(l.ctx, refundId)
		if refund != nil {
			refund.Status = int(pay.RefundStatus_RefundStatusSuccess)
			refund.RefundTime = time.Now().Unix()
			if err := l.svcCtx.RefundModel.Update(l.ctx, refund); err != nil {
				logx.Errorf("Refund: update refund status failed, refundId: %s, err: %v", refundId, err)
				return &pay.RefundResponse{
					RefundId: refundId,
					Success:  false,
					Message:  "refund status update failed",
				}, nil
			}
		}

		return &pay.RefundResponse{
			RefundId: refundId,
			Success:  true,
			Message:  "refund success",
		}, nil
	}

	return &pay.RefundResponse{
		RefundId: refundId,
		Success:  false,
		Message:  "refund failed",
	}, nil
}

func generateRefundId(nodeID int64) string {
	node, err := snowflake.NewNode(nodeID)
	if err != nil {
		return ""
	}
	return "REFUND_" + node.Generate().String()
}
