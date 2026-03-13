package logic

import (
	"context"
	"errors"
	"time"

	"github.com/jinzhu/gorm"
	"github.com/wansui976/go_zero_shop/apps/cart/rpc/cart"
	"github.com/wansui976/go_zero_shop/apps/cart/rpc/internal/svc"
	"go.uber.org/zap"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetCartListLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetCartListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetCartListLogic {
	return &GetCartListLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 获取购物车列表
func (l *GetCartListLogic) GetCartList(in *cart.CartListRequest) (*cart.CartListResponse, error) {
	// 1. 参数校验：用户ID必传
	if in.UserId <= 0 {
		return &cart.CartListResponse{
			Success: false,
			Code:    400,
			Message: "用户ID不能为空",
		}, nil
	}

	// 2. 查询用户有效购物车项（未删除：delete_status=0）
	cartItems, err := l.svcCtx.CartModel.FindByUserId(l.ctx, in.UserId)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			logx.Error("查询购物车列表失败", zap.Int64("userId", in.UserId), zap.Error(err))
			return &cart.CartListResponse{
				Success: false,
				Code:    500,
				Message: "系统错误：查询购物车失败",
			}, nil
		}
	}

	// 3. 数据统计与模型转换
	var (
		protoItems []*cart.CartItem // 转换为protobuf的购物车项列表
		totalCount int64            // 购物车商品总数（所有项）
		totalPrice int64            // 选中商品总价（分）
	)

	for _, item := range cartItems {
		// 转换数据库模型到protobuf模型
		protoItem := &cart.CartItem{
			Id:           item.Id,
			UserId:       item.UserId,
			ProductId:    item.ProductId,
			Price:        item.Price,
			Quantity:     item.Quantity,
			Selected:     item.Selected,
			DeleteStatus: item.DeleteStatus,
			CreateTime:   safeFormat(item.CreateTime),
			UpdateTime:   safeFormat(item.UpdateTime)}
		protoItems = append(protoItems, protoItem)

		// 统计总数（所有商品）
		totalCount += item.Quantity

		// 统计选中商品总价（单价*数量）
		if item.Selected == 1 {
			totalPrice += item.Price * item.Quantity
		}
	}

	// 4. 组装响应
	return &cart.CartListResponse{
		CartItems:  protoItems,
		TotalCount: totalCount,
		TotalPrice: totalPrice,
	}, nil
}
func safeFormat(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02 15:04:05")
}
