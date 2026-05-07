package logic

import (
	"context"

	"github.com/wansui976/go_zero_shop/apps/product/rpc/product"
	"github.com/wansui976/go_zero_shop/apps/seckill/rpc/internal/svc"
	"github.com/wansui976/go_zero_shop/apps/seckill/rpc/seckill"

	"github.com/zeromicro/go-zero/core/logx"
)

type SeckillProductsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSeckillProductsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SeckillProductsLogic {
	return &SeckillProductsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *SeckillProductsLogic) SeckillProducts(in *seckill.SeckillProductsRequest) (*seckill.SeckillProductsResponse, error) {
	resp, err := l.svcCtx.ProductRPC.ProductAllList(l.ctx, &product.ProductAllListRequest{})
	if err != nil {
		return nil, err
	}

	products := make([]*seckill.Product, 0, len(resp.Products))
	for _, p := range resp.Products {
		products = append(products, &seckill.Product{
			ProductId:  p.Id,
			Name:       p.Name,
			Desc:       p.Brief,
			Image:      p.ImageUrl,
			Stock:      p.Stock,
			CreateTime: p.CreateTime,
		})
	}

	return &seckill.SeckillProductsResponse{Products: products}, nil
}
