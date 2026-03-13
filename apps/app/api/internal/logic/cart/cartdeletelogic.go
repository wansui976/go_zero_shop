package cart

import (
	"context"

	"github.com/wansui976/go_zero_shop/apps/app/api/internal/middleware"
	"github.com/wansui976/go_zero_shop/apps/app/api/internal/svc"
	"github.com/wansui976/go_zero_shop/apps/app/api/internal/types"
	cart "github.com/wansui976/go_zero_shop/apps/cart/rpc/cart"

	"github.com/zeromicro/go-zero/core/logx"
)

type CartDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除用户购物车
func NewCartDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CartDeleteLogic {
	return &CartDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CartDeleteLogic) CartDelete(req *types.CartDeleteRequest) (resp *types.CommonResponse, err error) {
	// 获取当前用户ID
	uid, err := middleware.GetUserIDFromCtx(l.ctx)
	if err != nil {
		return &types.CommonResponse{
			ResultCode: 401,
			Msg:        "用户未登录或登录态失效",
			Data:       nil,
		}, nil
	}

	// 构建删除购物车请求
	deleteReq := &cart.DeleteCartRequest{
		UserId:    uid,
		ProductId: req.PID,
	}

	// 调用购物车RPC服务删除商品
	deleteResp, err := l.svcCtx.CartRPC.DeleteCart(l.ctx, deleteReq)
	if err != nil {
		l.Errorf("删除购物车失败: %v", err)
		return &types.CommonResponse{
			ResultCode: 500,
			Msg:        "删除购物车失败",
			Data:       nil,
		}, nil
	}

	// 返回结果
	return &types.CommonResponse{
		ResultCode: 200,
		Msg:        deleteResp.Message,
		Data:       nil,
	}, nil
}
