package logic

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/wansui976/go_zero_shop/apps/cart/rpc/cart"
	"github.com/wansui976/go_zero_shop/apps/cart/rpc/internal/svc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteCartLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDeleteCartLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteCartLogic {
	return &DeleteCartLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 删除购物车
func (l *DeleteCartLogic) DeleteCart(in *cart.DeleteCartRequest) (*cart.Response, error) {
	// 参数校验
	if in.UserId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "用户ID不能为空")
	}
	if in.ProductId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "商品ID不能为空")
	}

	// 查找购物车项
	cartItem, err := l.svcCtx.CartModel.FindOneByUserAndProduct(l.ctx, in.UserId, in.ProductId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "购物车项不存在")
		}
		logx.Error("查询购物车记录失败", logx.Field("userId", in.UserId), logx.Field("productId", in.ProductId), logx.Field("err", err))
		return nil, status.Error(codes.Internal, "系统错误：查询购物车失败")
	}

	// 执行逻辑删除：设置delete_status=1
	cartItem.DeleteStatus = 1
	cartItem.UpdateTime = time.Now()

	if err := l.svcCtx.CartModel.Update(l.ctx, cartItem); err != nil {
		logx.Error("更新购物车记录失败", logx.Field("cartId", cartItem.Id), logx.Field("err", err))
		return nil, status.Error(codes.Internal, "系统错误：删除购物车失败")
	}

	// 返回成功响应
	return &cart.Response{}, nil
}
