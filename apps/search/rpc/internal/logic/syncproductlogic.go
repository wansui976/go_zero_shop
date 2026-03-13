package logic

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/bytedance/sonic"
	"github.com/wansui976/go_zero_shop/apps/product/rpc/product"
	"github.com/wansui976/go_zero_shop/apps/search/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type SyncProductLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSyncProductLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SyncProductLogic {
	return &SyncProductLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// SyncProductToES 将 product 同步到 ES 索引（幂等）
func (l *SyncProductLogic) SyncProductToES(p *product.ProductItem) error {
	if p == nil {
		return fmt.Errorf("product is nil")
	}

	// 构建 ES 文档
	doc := map[string]interface{}{
		// ===== 基本信息 =====
		"id":          p.Id,
		"name":        p.Name,
		"brief":       p.Brief,
		"keywords":    p.Keywords,
		"image_url":   p.ImageUrl,

		// ===== 分类与品牌 =====
		"category_id":   p.CategoryId,
		"category_name": p.CategoryName,
		"category_ids":  p.CategoryIdList,
		"brand_id":      p.BrandId,
		"brand_name":    p.BrandName,

		// ===== 价格与库存 =====
		"price":    p.Price,
		"stock":    p.Stock,
		"low_stock": p.LowStock,
		"sales":    p.Sales,

		// ===== 物理属性 =====
		"unit":   p.Unit,
		"weight": p.Weight,

		// ===== 内容描述 =====
		"detail_title": p.DetailTitle,
		"detail_desc":  p.DetailDesc,
		"detail_html":  p.DetailHtml,

		// ===== 状态与控制 =====
		"sort":                p.Sort,
		"new_status_sort":     p.NewStatusSort,
		"recommend_status_sort": p.RecommendStatusSort,
		"status":              int64(p.Status),
		"create_time":         p.CreateTime,
		"update_time":         p.UpdateTime,
	}

	data, err := sonic.Marshal(doc)
	if err != nil {
		l.Errorf("marshal product doc failed: %v", err)
		return err
	}

	ctx, cancel := context.WithTimeout(l.ctx, 10*time.Second)
	defer cancel()

	// 使用 untyped API
	res, err := l.svcCtx.EsClient.Index(
		svc.IndexName,
		bytes.NewReader(data),
		l.svcCtx.EsClient.Index.WithDocumentID(strconv.FormatInt(p.Id, 10)),
		l.svcCtx.EsClient.Index.WithContext(ctx),
	)
	if err != nil {
		l.Errorf("es index request failed: %v", err)
		return err
	}
	defer res.Body.Close()

	if res.IsError() {
		bodyBytes, _ := io.ReadAll(res.Body)
		l.Errorf("es index returned error: %s, body: %s", res.Status(), string(bodyBytes))
		return fmt.Errorf("es index error: %s", res.Status())
	}

	l.Infof("product indexed to es success, id=%d", p.Id)
	return nil
}

// SyncProductsToES 批量同步商品到 ES（带重试）
func (l *SyncProductLogic) SyncProductsToES(products []*product.ProductItem) error {
	for _, p := range products {
		if err := l.SyncProductWithRetry(p); err != nil {
			l.Errorf("sync product %d failed: %v", p.Id, err)
			continue // 单个失败不影响其他
		}
	}
	return nil
}

// SyncProductWithRetry 带重试的同步
func (l *SyncProductLogic) SyncProductWithRetry(p *product.ProductItem) error {
	var lastErr error
	maxRetries := 3

	for i := 0; i < maxRetries; i++ {
		lastErr = l.SyncProductToES(p)
		if lastErr == nil {
			return nil
		}
		l.Errorf("sync product %d failed, retry %d/%d: %v", p.Id, i+1, maxRetries, lastErr)
		time.Sleep(time.Duration(i+1) * 100 * time.Millisecond)
	}

	return lastErr
}
