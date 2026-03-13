package cart

import (
	"context"

	"github.com/wansui976/go_zero_shop/apps/app/api/internal/middleware"
	"github.com/wansui976/go_zero_shop/apps/app/api/internal/svc"
	"github.com/wansui976/go_zero_shop/apps/app/api/internal/types"
	cart "github.com/wansui976/go_zero_shop/apps/cart/rpc/cart"

	"github.com/zeromicro/go-zero/core/logx"
)

type CartUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新购物车
func NewCartUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CartUpdateLogic {
	return &CartUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CartUpdateLogic) CartUpdate(req *types.CartUpdateRequest) (resp *types.CommonResponse, err error) {
	// 获取当前用户ID
	uid, err := middleware.GetUserIDFromCtx(l.ctx)
	if err != nil {
		return &types.CommonResponse{
			ResultCode: 401,
			Msg:        "用户未登录或登录态失效",
			Data:       nil,
		}, nil
	}

	// 构建更新购物车请求
	updateReq := &cart.UpdateCartRequest{
		UserId:    uid,
		ProductId: req.PID,
		Quantity:  req.Quantity,
		// Selected字段默认为0，如需更新选中状态，前端需单独传递
	}

	// 调用购物车RPC服务更新商品
	updateResp, err := l.svcCtx.CartRPC.UpdateCart(l.ctx, updateReq)
	if err != nil {
		l.Errorf("更新购物车失败: %v", err)
		return &types.CommonResponse{
			ResultCode: 500,
			Msg:        "更新购物车失败",
			Data:       nil,
		}, nil
	}

	// 返回结果
	return &types.CommonResponse{
		ResultCode: 200,
		Msg:        updateResp.Message,
		Data:       nil,
	}, nil
}
