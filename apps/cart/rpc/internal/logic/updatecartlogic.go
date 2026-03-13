package logic

import (
	"context"
	"database/sql"
	"errors"
	"github.com/wansui976/go_zero_shop/apps/cart/rpc/cart"
	"github.com/wansui976/go_zero_shop/apps/cart/rpc/internal/svc"
	"go.uber.org/zap"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateCartLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateCartLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateCartLogic {
	return &UpdateCartLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// UpdateCart 更新购物车（支持数量/选中状态单独或批量更新）
func (l *UpdateCartLogic) UpdateCart(in *cart.UpdateCartRequest) (*cart.Response, error) {
	// 1. 基础参数校验：用户ID和商品ID必传
	if in.UserId <= 0 {
		return &cart.Response{
			Success: false,
			Code:    400,
			Message: "用户ID不能为空",
		}, nil
	}
	if in.ProductId <= 0 {
		return &cart.Response{
			Success: false,
			Code:    400,
			Message: "商品ID不能为空",
		}, nil
	}

	// 2. 可选参数合法性校验
	updateFields := make([]string, 0) // 记录需要更新的字段
	if in.Quantity > 0 {
		updateFields = append(updateFields, "quantity")
	} else if in.Quantity < 0 {
		return &cart.Response{
			Success: false,
			Code:    400,
			Message: "商品数量不能为负数",
		}, nil
	}

	if in.Selected == 0 || in.Selected == 1 {
		updateFields = append(updateFields, "selected")
	} else if in.Selected != 0 { // 排除默认值0的情况（未传参时）
		return &cart.Response{
			Success: false,
			Code:    400,
			Message: "选中状态只能是0（未选中）或1（选中）",
		}, nil
	}

	// 3. 无更新字段时直接返回
	if len(updateFields) == 0 {
		return &cart.Response{
			Success: false,
			Code:    400,
			Message: "请传入需要更新的字段（数量或选中状态）",
		}, nil
	}

	// 4. 查询购物车中有效记录（未删除）
	cartItem, err := l.svcCtx.CartModel.FindOneByUserAndProduct(l.ctx, in.UserId, in.ProductId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &cart.Response{
				Success: false,
				Code:    404,
				Message: "该商品不在购物车中，无法更新",
			}, nil
		}
		logx.Error("查询购物车记录失败", zap.Int64("userId", in.UserId), zap.Int64("productId", in.ProductId), zap.Error(err))
		return &cart.Response{
			Success: false,
			Code:    500,
			Message: "系统错误：查询购物车失败",
		}, nil
	}

	// 5. 选择性更新字段
	if in.Quantity > 0 {
		cartItem.Quantity = int64(in.Quantity)
	}
	if in.Selected == 0 || in.Selected == 1 {
		cartItem.Selected = int64(in.Selected)
	}
	cartItem.UpdateTime = time.Now() // 同步更新时间

	// 6. 执行数据库更新
	if err := l.svcCtx.CartModel.Update(l.ctx, cartItem); err != nil {
		logx.Error("更新购物车记录失败", zap.Int64("cartId", cartItem.Id), zap.Error(err))
		return &cart.Response{
			Success: false,
			Code:    500,
			Message: "系统错误：更新购物车失败",
		}, nil
	}

	// 7. 构造成功响应
	return &cart.Response{
		Success: true,
	}, nil
}
