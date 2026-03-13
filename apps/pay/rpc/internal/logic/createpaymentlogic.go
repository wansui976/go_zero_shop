package logic

import (
	"context"
	"time"

	"github.com/bwmarrin/snowflake"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/wansui976/go_zero_shop/apps/pay/rpc/internal/svc"
	"github.com/wansui976/go_zero_shop/apps/pay/rpc/pay"
	"github.com/wansui976/go_zero_shop/apps/pay/rpc/model"
)

type CreatePaymentLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreatePaymentLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreatePaymentLogic {
	return &CreatePaymentLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreatePaymentLogic) CreatePayment(in *pay.CreatePaymentRequest) (*pay.CreatePaymentResponse, error) {
	// 1. 生成支付单号
	paymentId := generatePaymentId(l.svcCtx.Config.Snowflake.NodeID)
	
	_, err := l.svcCtx.PaymentModel.Insert(l.ctx, &model.Payment{
		PaymentId:     paymentId,
		OrderId:       in.OrderId,
		UserId:        in.UserId,
		Amount:        in.Amount,
		Status:        int(pay.PaymentStatus_PaymentStatusPending),
		PaymentType:   int(in.PaymentType),
		TransactionId: "",
		CreateTime:    time.Now().Unix(),
		PayTime:       0,
	})
	if err != nil {
		logx.Errorf("CreatePayment: insert payment failed, err: %v", err)
		return nil, err
	}

	// 2. 生成支付链接（这里简化，实际需要调用第三方支付API）
	payUrl := generatePayUrl(in.PaymentType, in.OrderId, in.Amount)

	// 3. 设置过期时间（默认30分钟）
	expireTime := time.Now().Add(30 * time.Minute).Unix()

	return &pay.CreatePaymentResponse{
		PaymentId:  paymentId,
		PayUrl:     payUrl,
		ExpireTime: expireTime,
	}, nil
}

// 生成支付单号
func generatePaymentId(nodeID int64) string {
	node, err := snowflake.NewNode(nodeID)
	if err != nil {
		return ""
	}
	return node.Generate().String()
}

// 生成支付链接（模拟）
func generatePayUrl(paymentType pay.PaymentType, orderId string, amount int64) string {
	switch paymentType {
	case pay.PaymentType_PaymentTypeWechat:
		return "weixin://wxpay/bizpayurl?pr=mock_" + orderId
	case pay.PaymentType_PaymentTypeAlipay:
		return "alipay://alipay.com?out_trade_no=" + orderId
	default:
		return ""
	}
}
