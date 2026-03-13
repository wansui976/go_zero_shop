package product

import (
	"context"

	"github.com/wansui976/go_zero_shop/apps/app/api/internal/svc"
	"github.com/wansui976/go_zero_shop/apps/app/api/internal/types"
	"github.com/wansui976/go_zero_shop/apps/product/rpc/productclient"

	"github.com/zeromicro/go-zero/core/logx"
)

type ProductDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 商品详情
func NewProductDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProductDetailLogic {
	return &ProductDetailLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ProductDetailLogic) ProductDetail(req *types.ProductDetailRequest) (resp *types.CommonResponse, err error) {
	var request productclient.ProductItemRequest
	request.ProductId = req.ProductID
	response, err := l.svcCtx.ProductRPC.Product(l.ctx, &request)
	if err != nil {
		return nil, err
	}

	var product types.Product
	product = types.Product{
		Id:                  response.Id,
		Name:                response.Name,
		Brief:               response.Brief,
		Keywords:            response.Keywords,
		ImageUrl:            response.ImageUrl,
		CategoryId:          response.CategoryId,
		CategoryName:        response.CategoryName,
		CategoryIds:         response.CategoryIdList,
		BrandId:             response.BrandId,
		BrandName:           response.BrandName,
		Price:               response.Price,
		Stock:               response.Stock,
		LowStock:            int64(response.LowStock),
		Sales:               int64(response.Sales),
		Unit:                response.Unit,
		Weight:              float64(response.Weight),
		DetailTitle:         response.DetailTitle,
		DetailDesc:          response.DetailDesc,
		DetailHtml:          response.DetailHtml,
		Sort:                int64(response.Sort),
		NewStatusSort:       int64(response.NewStatusSort),
		RecommendStatusSort: int64(response.RecommendStatusSort),
		Status:              int64(response.Status),
		CreateTime:          response.CreateTime,
		UpdateTime:          response.UpdateTime,
	}

	resp = &types.CommonResponse{
		ResultCode: 200,
		Msg:        "success",
		Data:       product,
	}
	return resp, nil
}
