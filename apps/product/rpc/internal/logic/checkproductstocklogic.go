package logic

import (
	"context"
	"errors"

	"github.com/dtm-labs/dtmcli"
	"github.com/wansui976/go_zero_shop/apps/product/rpc/internal/svc"
	"github.com/wansui976/go_zero_shop/apps/product/rpc/product"
	"github.com/wansui976/go_zero_shop/apps/product/rpc/internal/model"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type CheckProductStockLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCheckProductStockLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CheckProductStockLogic {
	return &CheckProductStockLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CheckProductStockLogic) CheckProductStock(in *product.UpdateProductStockRequest) (*product.UpdateProductStockResponse, error) {
	if in.ProductId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "无效的商品id")
	}
	if in.Num <= 0 {
		return nil, status.Error(codes.InvalidArgument, "请求数量必须大于 0")
	}

	l.Info("检查商品库存,productId:%d,需要数量%d", in.ProductId, in.Num)

	// 查询商品信息
	p, err := l.svcCtx.ProductModel.FindOne(l.ctx, in.ProductId)
	if err != nil {
		// 区分商品不存在和其他错误
		if errors.Is(err, model.ErrNotFound) {
			l.Errorf("商品不存在, productId: %d", in.ProductId)
			return nil, status.Error(codes.NotFound, "商品不存在")
		}
		l.Errorf("查询商品失败, productId: %d, err: %v", in.ProductId, err)
		return nil, status.Error(codes.Internal, "查询商品信息失败")
	}

	// 检查商品状态是否可用
	if p.Status != 1 { // 假设1表示在售状态
		l.Errorf("商品不可售, productId: %d, status: %d", in.ProductId, p.Status)
		return nil, status.Error(codes.FailedPrecondition, "商品不可售")
	}

	// 检查库存是否充足
	if p.Stock < in.Num {
		l.Errorf("商品库存不足, productId: %d, 库存: %d, 需要: %d",
			in.ProductId, p.Stock, in.Num)
		return nil, status.Error(codes.ResourceExhausted, dtmcli.ResultFailure)
	}

	l.Infof("商品库存检查通过, productId: %d, 库存: %d, 需要: %d",
		in.ProductId, p.Stock, in.Num)
	return &product.UpdateProductStockResponse{}, nil
}
