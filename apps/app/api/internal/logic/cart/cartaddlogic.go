package cart

import (
	"context"

	"github.com/wansui976/go_zero_shop/apps/app/api/internal/middleware"
	"github.com/wansui976/go_zero_shop/apps/app/api/internal/svc"
	"github.com/wansui976/go_zero_shop/apps/app/api/internal/types"
	cart "github.com/wansui976/go_zero_shop/apps/cart/rpc/cart"

	"github.com/zeromicro/go-zero/core/logx"
)

type CartAddLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 添加购物车
func NewCartAddLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CartAddLogic {
	return &CartAddLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CartAddLogic) CartAdd(req *types.CartAddRequest) (resp *types.CommonResponse, err error) {
	var request cart.CartItemRequest
	request.Price = req.Price
	request.ProductId = req.PID
	request.UserId, err = middleware.GetUserIDFromCtx(l.ctx)
	if err != nil {
		return &types.CommonResponse{
			ResultCode: 401,
			Msg:        "用户未登录或登录态失效",
			Data:       nil,
		}, nil
	}
	// 设置默认值
	request.Quantity = req.Quantity
	request.Selected = 1
	_, err = l.svcCtx.CartRPC.AddCart(l.ctx, &request)
	if err != nil {

		l.Errorf("添加购物车失败: %v", err)
		return &types.CommonResponse{
			ResultCode: 500,
			Msg:        "添加购物车失败",
			Data:       nil,
		}, nil
	}

	resp = &types.CommonResponse{
		ResultCode: 200,
		Msg:        "success",
		Data:       nil,
	}

	return resp, nil
}
