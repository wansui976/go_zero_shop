package home

import (
	"context"

	"github.com/wansui976/go_zero_shop/apps/app/api/internal/svc"
	"github.com/wansui976/go_zero_shop/apps/app/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type FlashSaleLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 限时抢购商品列表
func NewFlashSaleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FlashSaleLogic {
	return &FlashSaleLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FlashSaleLogic) FlashSale() (resp *types.CommonResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
