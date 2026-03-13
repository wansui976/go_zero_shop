package cart

import (
	"context"

	"github.com/wansui976/go_zero_shop/apps/app/api/internal/middleware"
	"github.com/wansui976/go_zero_shop/apps/app/api/internal/svc"
	"github.com/wansui976/go_zero_shop/apps/app/api/internal/types"
	cart "github.com/wansui976/go_zero_shop/apps/cart/rpc/cart"

	"github.com/zeromicro/go-zero/core/logx"
)

type CartListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 购物车商品列表
func NewCartListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CartListLogic {
	return &CartListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CartListLogic) CartList(req *types.CartListRequest) (resp *types.CommonResponse, err error) {
	// 获取当前用户ID
	uid, err := middleware.GetUserIDFromCtx(l.ctx)
	if err != nil {
		return &types.CommonResponse{
			ResultCode: 401,
			Msg:        "用户未登录或登录态失效",
			Data:       nil,
		}, nil
	}

	// 构建获取购物车列表请求
	listReq := &cart.CartListRequest{
		UserId: uid,
	}

	// 调用购物车RPC服务获取购物车列表
	listResp, err := l.svcCtx.CartRPC.GetCartList(l.ctx, listReq)
	if err != nil {
		l.Errorf("获取购物车列表失败: %v", err)
		return &types.CommonResponse{
			ResultCode: 500,
			Msg:        "获取购物车列表失败",
			Data:       nil,
		}, nil
	}

	// 返回结果
	return &types.CommonResponse{
		ResultCode: 200,
		Msg:        listResp.Message,
		Data:       listResp,
	}, nil
}
