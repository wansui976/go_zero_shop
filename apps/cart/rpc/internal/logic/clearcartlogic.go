package logic

import (
	"context"

	"github.com/wansui976/go_zero_shop/apps/cart/rpc/cart"
	"github.com/wansui976/go_zero_shop/apps/cart/rpc/internal/svc"
	"github.com/zeromicro/go-zero/core/logx"
)

type ClearCartLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewClearCartLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ClearCartLogic {
	return &ClearCartLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ClearCart 清空用户购物车（逻辑删除：批量置delete_status=1）
func (l *ClearCartLogic) ClearCart(in *cart.CartListRequest) (*cart.Response, error) {
	// // 1. 参数校验：用户ID必传
	// if in.UserId <= 0 {
	// 	return &cart.Response{
	// 		Success: false,
	// 		Code:    400,
	// 		Message: "用户ID不能为空",
	// 	}, nil
	// }

	// // 2. 批量更新用户下所有有效购物车项（delete_status=0 → 1）
	// result, err := l.svcCtx.CartModel.Delete()
	// result, err := l.svcCtx.CartModel.UpdateByUserId(l.ctx, in.UserId)
	// if err != nil {
	// 	logx.Error("清空购物车失败", zap.Int64("userId", in.UserId), zap.Error(err))
	// 	return &cart.Response{
	// 		Success: false,
	// 		Code:    500,
	// 		Message: "系统错误：清空购物车失败",
	// 	}, nil
	// }

	// // 3. 处理更新结果（无数据时友好提示）
	// if result == 0 {
	// 	return &cart.Response{
	// 		Success: true,
	// 		Code:    200,
	// 		Message: "购物车已为空，无需清空",
	// 	}, nil
	// }

	// // 4. 成功响应
	// return &cart.Response{
	// 	Success: true,
	// 	Code:    200,
	// 	Message: "购物车清空成功",
	// }, nil
	return nil, nil
}
