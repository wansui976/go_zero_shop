package logic

import (
	"context"
	"github.com/wansui976/go_zero_shop/apps/cart/rpc/cart"
	"github.com/wansui976/go_zero_shop/apps/cart/rpc/internal/svc"
	"github.com/zeromicro/go-zero/core/logx"
)

type CheckCartLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCheckCartLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CheckCartLogic {
	return &CheckCartLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// CheckCart 切换/设置购物车商品选中状态
func (l *CheckCartLogic) CheckCart(in *cart.CartItemRequest) (*cart.Response, error) {
	return nil, nil
}
