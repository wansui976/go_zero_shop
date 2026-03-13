package logic

import (
	"context"

	"github.com/wansui976/go_zero_shop/internal/svc"
	"github.com/wansui976/go_zero_shop/pay"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetRefundByIdLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetRefundByIdLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetRefundByIdLogic {
	return &GetRefundByIdLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 查询退款状态
func (l *GetRefundByIdLogic) GetRefundById(in *pay.GetRefundByIdRequest) (*pay.GetRefundByIdResponse, error) {
	// todo: add your logic here and delete this line

	return &pay.GetRefundByIdResponse{}, nil
}
