package payclient

import (
	"context"

	"github.com/wansui976/go_zero_shop/apps/pay/rpc/pay"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
)

type (
	CreatePaymentReq  = pay.CreatePaymentRequest
	CreatePaymentResp = pay.CreatePaymentResponse
	
	GetPaymentByOrderIdReq  = pay.GetPaymentByOrderIdRequest
	GetPaymentByOrderIdResp = pay.GetPaymentByOrderIdResponse
	
	PaymentCallbackReq  = pay.PaymentCallbackRequest
	PaymentCallbackResp = pay.PaymentCallbackResponse
	
	RefundReq  = pay.RefundRequest
	RefundResp = pay.RefundResponse
	
	GetRefundByIdReq  = pay.GetRefundByIdRequest
	GetRefundByIdResp = pay.GetRefundByIdResponse

	Pay interface {
		CreatePayment(ctx context.Context, in *CreatePaymentReq, opts ...grpc.CallOption) (*CreatePaymentResp, error)
		GetPaymentByOrderId(ctx context.Context, in *GetPaymentByOrderIdReq, opts ...grpc.CallOption) (*GetPaymentByOrderIdResp, error)
		PaymentCallback(ctx context.Context, in *PaymentCallbackReq, opts ...grpc.CallOption) (*PaymentCallbackResp, error)
		Refund(ctx context.Context, in *RefundReq, opts ...grpc.CallOption) (*RefundResp, error)
		GetRefundById(ctx context.Context, in *GetRefundByIdReq, opts ...grpc.CallOption) (*GetRefundByIdResp, error)
	}

	defaultPay struct {
		cli zrpc.Client
	}
)

func NewPay(cli zrpc.Client) Pay {
	return &defaultPay{
		cli: cli,
	}
}

func NewPayByTarget(target string, opts ...zrpc.ClientOption) Pay {
	return &defaultPay{
		cli: zrpc.MustNewClient(zrpc.RpcClientConf{Target: target}, opts...),
	}
}

func (m *defaultPay) CreatePayment(ctx context.Context, in *CreatePaymentReq, opts ...grpc.CallOption) (*CreatePaymentResp, error) {
	client := pay.NewPayServiceClient(m.cli.Conn())
	return client.CreatePayment(ctx, in, opts...)
}

func (m *defaultPay) GetPaymentByOrderId(ctx context.Context, in *GetPaymentByOrderIdReq, opts ...grpc.CallOption) (*GetPaymentByOrderIdResp, error) {
	client := pay.NewPayServiceClient(m.cli.Conn())
	return client.GetPaymentByOrderId(ctx, in, opts...)
}

func (m *defaultPay) PaymentCallback(ctx context.Context, in *PaymentCallbackReq, opts ...grpc.CallOption) (*PaymentCallbackResp, error) {
	client := pay.NewPayServiceClient(m.cli.Conn())
	return client.PaymentCallback(ctx, in, opts...)
}

func (m *defaultPay) Refund(ctx context.Context, in *RefundReq, opts ...grpc.CallOption) (*RefundResp, error) {
	client := pay.NewPayServiceClient(m.cli.Conn())
	return client.Refund(ctx, in, opts...)
}

func (m *defaultPay) GetRefundById(ctx context.Context, in *GetRefundByIdReq, opts ...grpc.CallOption) (*GetRefundByIdResp, error) {
	client := pay.NewPayServiceClient(m.cli.Conn())
	return client.GetRefundById(ctx, in, opts...)
}
