package home

import (
	"context"

	"github.com/wansui976/go_zero_shop/apps/app/api/internal/svc"
	"github.com/wansui976/go_zero_shop/apps/app/api/internal/types"
	"github.com/wansui976/go_zero_shop/apps/product/rpc/product"

	"github.com/zeromicro/go-zero/core/logx"
)

type HomeLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取首页信息
func NewHomeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *HomeLogic {
	return &HomeLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *HomeLogic) Home() (resp *types.CommonResponse, err error) {
	// 1. 轮播图：category_id = 10
	carouselResp, err := l.svcCtx.ProductRPC.ProductList(l.ctx, &product.ProductListRequest{
		CategoryId: 10,
		Cursor:     0, // 不分页
		Ps:         5, // 获取5条
	})
	if err != nil {
		return nil, err
	}

	carouselList := make([]*types.ProductItem, 0)
	for _, c := range carouselResp.Products {
		carouselList = append(carouselList, &types.ProductItem{
			GoodsId:       c.Id,
			GoodsName:     c.Name,
			SellingPrice:  c.Price,
			GoodsCoverImg: c.ImageUrl,
		})
	}

	// 2. 新品：category_id = 11

	newResp, err := l.svcCtx.ProductRPC.ProductList(l.ctx, &product.ProductListRequest{
		CategoryId: 11,
		Cursor:     0,
		Ps:         10,
	})
	if err != nil {
		return nil, err
	}

	newGoodses := make([]*types.ProductItem, 0)
	for _, p := range newResp.Products {
		newGoodses = append(newGoodses, &types.ProductItem{
			GoodsId:       p.Id,
			GoodsName:     p.Name,
			SellingPrice:  p.Price,
			GoodsCoverImg: p.ImageUrl,
		})
	}

	// 3. 热门商品：category_id = 12

	hotResp, err := l.svcCtx.ProductRPC.ProductList(l.ctx, &product.ProductListRequest{
		CategoryId: 12,
		Cursor:     0,
		Ps:         10,
	})
	if err != nil {
		return nil, err
	}

	hotGoodses := make([]*types.ProductItem, 0)
	for _, p := range hotResp.Products {
		hotGoodses = append(hotGoodses, &types.ProductItem{
			GoodsId:       p.Id,
			GoodsName:     p.Name,
			SellingPrice:  p.Price,
			GoodsCoverImg: p.ImageUrl,
		})
	}

	return &types.CommonResponse{
		ResultCode: 200,
		Msg:        "success",
		Data: &types.HomeResponse{
			CarouselList: carouselList,
			NewGoodses:   newGoodses,
			HotGoodses:   hotGoodses,
		},
	}, nil
}
