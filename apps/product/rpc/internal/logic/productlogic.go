package logic

import (
	"context"
	"fmt"

	"github.com/wansui976/go_zero_shop/apps/product/rpc/internal/model"
	"github.com/wansui976/go_zero_shop/apps/product/rpc/internal/svc"
	"github.com/wansui976/go_zero_shop/apps/product/rpc/product"
	"github.com/zeromicro/go-zero/core/logx"
)

type ProductLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewProductLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProductLogic {
	return &ProductLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 这段代码实现了一个单个商品详情查询的业务逻辑，核心功能是根据商品 ID 获取商品信息，
// 并通过 SingleGroup 实现单机级别的缓存防穿透（避免同一时间大量重复请求穿透到数据库）
func (l *ProductLogic) Product(in *product.ProductItemRequest) (*product.ProductItem, error) {
	v, err, _ := l.svcCtx.SingleGroup.Do(fmt.Sprintf("product:%d", in.ProductId), func() (interface{}, error) {
		return l.svcCtx.ProductModel.FindOne(l.ctx, in.ProductId)
	})
	if err != nil {
		return nil, err
	}
	p := v.(*model.Product)
	return &product.ProductItem{
		// ===== 基本信息 =====
		Id:       p.Id,
		Name:     p.Name,
		Brief:    p.Brief.String,
		Keywords: p.Keywords.String,
		ImageUrl: p.ImageUrl.String,

		// ===== 分类与品牌 =====
		CategoryId:     p.CategoryId.Int64,
		CategoryName:   p.CategoryName.String,
		CategoryIdList: p.CategoryIds.String,
		BrandId:        p.BrandId.Int64,
		BrandName:      p.BrandName.String,

		// ===== 价格与库存 =====
		Price:    p.Price,
		Stock:    p.Stock,
		LowStock: int32(p.LowStock),
		Sales:    int32(p.Sales),

		// ===== 物理属性 =====
		Unit:   p.Unit.String,
		Weight: float32(p.Weight),

		// ===== 内容描述 =====
		DetailTitle: p.DetailTitle.String,
		DetailDesc:  p.DetailDesc.String,
		DetailHtml:  p.DetailHtml.String,

		// ===== 状态与控制 =====
		Sort:                int32(p.Sort),
		NewStatusSort:       int32(p.NewStatusSort),
		RecommendStatusSort: int32(p.RecommendStatusSort),
		Status:              product.ProductStatus(p.Status),

		// ===== 时间戳（毫秒）=====
		CreateTime: p.CreateTime.UnixMilli(),
		UpdateTime: p.UpdateTime.UnixMilli(),
	}, nil
}
