package logic

import (
	"context"

	"github.com/wansui976/go_zero_shop/apps/product/rpc/internal/svc"
	"github.com/wansui976/go_zero_shop/apps/product/rpc/product"

	"github.com/zeromicro/go-zero/core/logx"
)

type OperationProductsLogic struct {
	ctx              context.Context
	svcCtx           *svc.ServiceContext
	productListLogic *ProductListLogic
	logx.Logger
}

func NewOperationProductsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OperationProductsLogic {
	return &OperationProductsLogic{
		ctx:              ctx,
		svcCtx:           svcCtx,
		productListLogic: NewProductListLogic(ctx, svcCtx),
		Logger:           logx.WithContext(ctx),
	}
}

const (
	validStatus          = 1
	operationProductsKey = "operation#products"
)

// 获取运营商品列表
func (l *OperationProductsLogic) OperationProducts(in *product.OperationProductsRequest) (*product.OperationProductsResponse, error) {
	opProducts, ok := l.svcCtx.LocalCache.Get(operationProductsKey)
	if ok {
		return &product.OperationProductsResponse{Products: opProducts.([]*product.ProductItem)}, nil
	}

	pos, err := l.svcCtx.OperationModel.OperationProducts(l.ctx, validStatus)
	if err != nil {
		return nil, err
	}

	pids := make([]int64, 0, len(pos))
	for _, p := range pos {
		pids = append(pids, p.ProductId)
	}

	products, err := l.productListLogic.productsByIds(l.ctx, pids)
	if err != nil {
		return nil, err
	}

	pItems := make([]*product.ProductItem, 0, len(products))
	for _, p := range products {
		brief := ""
		if p.Brief.Valid {
			brief = p.Brief.String
		}
		img := ""
		if p.ImageUrl.Valid {
			img = p.ImageUrl.String
		}
		pItems = append(pItems, &product.ProductItem{
			Id:         p.Id,
			Name:       p.Name,
			Brief:      brief,
			ImageUrl:   img,
			Price:      p.Price,
			Stock:      p.Stock,
			CategoryId: p.CategoryId.Int64,
			BrandId:    p.BrandId.Int64,
			Status:     product.ProductStatus(p.Status),
			CreateTime: p.CreateTime.Unix(),
		})
	}
	l.svcCtx.LocalCache.Set(operationProductsKey, pItems)
	return &product.OperationProductsResponse{Products: pItems}, nil
}
