package logic

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/wansui976/go_zero_shop/apps/cart/rpc/cart"
	"github.com/wansui976/go_zero_shop/apps/cart/rpc/internal/svc"
	"github.com/wansui976/go_zero_shop/apps/cart/rpc/model"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/zeromicro/go-zero/core/logx"
)

type AddCartLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAddCartLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AddCartLogic {
	return &AddCartLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 添加购物车
func (l *AddCartLogic) AddCart(in *cart.CartItemRequest) (*cart.Response, error) {

	if in.ProductId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "商品ID不能为空")
	}
	if in.Price <= 0 {
		return nil, status.Error(codes.InvalidArgument, "商品价格不能为空")
	}
	if in.Quantity <= 0 {
		in.Quantity = 1 // 数量默认1
	}
	if in.Selected < 0 || in.Selected > 1 {
		in.Selected = 1 // 选中状态默认1
	}

	// 3. 查询购物车中是否已有该用户+商品的有效记录（未删除）
	existingItem, err := l.svcCtx.CartModel.FindOneByUserAndProduct(l.ctx, in.UserId, in.ProductId)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		// 数据库查询错误
		logx.Error("查询购物车记录失败", logx.Field("err", err))
		return nil, status.Error(codes.Internal, "系统错误：查询购物车失败")
	}

	// 4. 存在则更新数量，不存在则新增
	if existingItem != nil {
		// 已有记录：更新数量（累加或覆盖，这里选择累加）
		existingItem.Quantity += in.Quantity
		existingItem.Price = in.Price // 同步最新价格（可选，根据业务需求）
		existingItem.Selected = int64(in.Selected)
		existingItem.UpdateTime = time.Now()

		if err := l.svcCtx.CartModel.Update(l.ctx, existingItem); err != nil {
			logx.Errorf("更新购物车记录失败: %v", err)
			return nil, status.Error(codes.Internal, "系统错误：更新购物车失败")
		}

		return &cart.Response{}, nil
	} else {
		// 无记录：新增购物车项
		newItem := &model.Cart{
			UserId:       in.UserId,
			ProductId:    in.ProductId,
			Price:        in.Price,
			Quantity:     in.Quantity,
			Selected:     int64(in.Selected),
			DeleteStatus: 0, // 正常状态
			CreateTime:   time.Now(),
			UpdateTime:   time.Now(),
		}

		if _, err := l.svcCtx.CartModel.Insert(l.ctx, newItem); err != nil {
			logx.Errorf("新增购物车记录失败: %v", err)
			return nil, status.Error(codes.Internal, "系统错误：添加购物车失败")
		}

		return &cart.Response{}, nil
	}
}
