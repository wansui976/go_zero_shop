package logic

import (
	"context"

	"github.com/wansui976/go_zero_shop/apps/product/rpc/internal/svc"
	"github.com/wansui976/go_zero_shop/apps/product/rpc/product"

	"github.com/zeromicro/go-zero/core/logx"
)

type ProductAllListLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewProductAllListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProductAllListLogic {
	return &ProductAllListLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ProductAllListLogic) ProductAllList(in *product.ProductAllListRequest) (*product.ProductAllListResponse, error) {

	productsFromDB, err := l.svcCtx.ProductModel.GetAll(l.ctx)
	if err != nil {
		l.Errorf("ProductAllList failed: %v", err)
		return nil, err
	}
	products := make([]*product.ProductItem, 0, len(productsFromDB))
	for _, p := range productsFromDB {
		brief := ""
		if p.Brief.Valid {
			brief = p.Brief.String
		}
		img := ""
		if p.ImageUrl.Valid {
			img = p.ImageUrl.String
		}
		products = append(products, &product.ProductItem{
			Id:                  p.Id,
			Name:                p.Name,
			Brief:               brief,
			Keywords:            p.Keywords.String,
			ImageUrl:            img,
			Price:               p.Price,
			Sales:               int32(p.Sales),
			DetailDesc:          p.DetailDesc.String,
			DetailHtml:          p.DetailHtml.String,
			NewStatusSort:       int32(p.NewStatusSort),
			RecommendStatusSort: int32(p.RecommendStatusSort),
			CategoryId:          p.CategoryId.Int64,
			CategoryName:        p.CategoryName.String,
			BrandId:             p.BrandId.Int64,
			BrandName:           p.BrandName.String,
			Status:              product.ProductStatus(p.Status),
			CreateTime:          p.CreateTime.Unix(),
		})
	}

	return &product.ProductAllListResponse{
		Products: products,
	}, nil
}
