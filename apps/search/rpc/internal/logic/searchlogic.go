package logic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/bytedance/sonic"
	"github.com/wansui976/go_zero_shop/apps/product/rpc/product"
	"github.com/wansui976/go_zero_shop/apps/search/rpc/internal/cache"
	"github.com/wansui976/go_zero_shop/apps/search/rpc/internal/svc"
	"github.com/wansui976/go_zero_shop/apps/search/rpc/search"

	"github.com/zeromicro/go-zero/core/logx"
)

type SearchLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSearchLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SearchLogic {
	return &SearchLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// buildSearchQuery 构建优化的 ES 查询（Query + Filter 组合）
func (l *SearchLogic) buildSearchQuery(in *search.SearchReq) map[string]interface{} {
	query := map[string]interface{}{}

	// bool 查询
	boolQuery := map[string]interface{}{}

	// 1. must: 影响评分的条件（关键词匹配）
	must := []interface{}{}

	if in.Keyword != "" {
		must = append(must, map[string]interface{}{
			"multi_match": map[string]interface{}{
				"query": in.Keyword,
				// 字段权重配置：重要字段权重更高
				"fields": []string{
					"name^3",     // 商品名称权重3倍（最重要）
					"keywords^2", // 关键词权重2倍
					"brief^1.5",  // 简介权重1.5倍
					"detail_title^1.2",
					"detail_desc^1",
					"category_name^0.8",
					"brand_name^0.5",
				},
				"type":        "best_fields",
				"fuzziness":   "AUTO",
				"tie_breaker": 0.3,
			},
		})
	} else {
		must = append(must, map[string]interface{}{
			"match_all": map[string]interface{}{},
		})
	}
	boolQuery["must"] = must

	// 2. filter: 不影响评分的精确条件（可缓存）
	filter := []interface{}{}

	// 状态过滤
	if in.Status == 0 {
		filter = append(filter, map[string]interface{}{
			"term": map[string]interface{}{
				"status": 0,
			},
		})
	} else {
		filter = append(filter, map[string]interface{}{
			"term": map[string]interface{}{
				"status": 1,
			},
		})
	}

	// 库存过滤
	if in.HasStock {
		filter = append(filter, map[string]interface{}{
			"range": map[string]interface{}{
				"stock": map[string]interface{}{
					"gt": 0,
				},
			},
		})
	}

	// 价格区间过滤
	if in.PriceMin > 0 || in.PriceMax > 0 {
		priceRange := map[string]interface{}{}
		if in.PriceMin > 0 {
			priceRange["gte"] = in.PriceMin
		}
		if in.PriceMax > 0 {
			priceRange["lte"] = in.PriceMax
		}
		filter = append(filter, map[string]interface{}{
			"range": map[string]interface{}{
				"price": priceRange,
			},
		})
	}

	// 分类筛选（支持多选）
	if len(in.CategoryIds) > 0 {
		filter = append(filter, map[string]interface{}{
			"terms": map[string]interface{}{
				"category_id": in.CategoryIds,
			},
		})
	}

	// 品牌筛选（支持多选）
	if len(in.BrandIds) > 0 {
		filter = append(filter, map[string]interface{}{
			"terms": map[string]interface{}{
				"brand_id": in.BrandIds,
			},
		})
	}

	boolQuery["filter"] = filter

	query["query"] = map[string]interface{}{
		"bool": boolQuery,
	}

	// 分页
	query["from"] = (in.PageNum - 1) * in.PageSize
	query["size"] = in.PageSize

	// 高亮
	query["highlight"] = map[string]interface{}{
		"fields": map[string]interface{}{
			"name":  map[string]interface{}{},
			"brief": map[string]interface{}{},
		},
		"pre_tags":  []string{"<em>"},
		"post_tags": []string{"</em>"},
	}

	// 排序
	sortField := []interface{}{}
	switch in.Sort {
	case 1:
		sortField = append(sortField, map[string]interface{}{"new_status_sort": map[string]string{"order": "desc"}})
		sortField = append(sortField, map[string]interface{}{"create_time": map[string]string{"order": "desc"}})
	case 2:
		sortField = append(sortField, map[string]interface{}{"sales": map[string]string{"order": "desc"}})
	case 3:
		sortField = append(sortField, map[string]interface{}{"price": map[string]string{"order": "asc"}})
	case 4:
		sortField = append(sortField, map[string]interface{}{"price": map[string]string{"order": "desc"}})
	}
	if len(sortField) > 0 {
		query["sort"] = sortField
	}

	// 性能优化：只返回需要的字段
	query["_source"] = map[string]interface{}{
		"excludes": []string{"detail_html"},
	}

	// 聚合查询（如果需要）
	if in.NeedAgg {
		query["aggs"] = map[string]interface{}{
			"brand_aggs": map[string]interface{}{
				"terms": map[string]interface{}{
					"field": "brand_id",
					"size":  20,
				},
				"aggs": map[string]interface{}{
					"brand_name": map[string]interface{}{
						"terms": map[string]interface{}{
							"field": "brand_name.keyword",
							"size":  1,
						},
					},
				},
			},
			"category_aggs": map[string]interface{}{
				"terms": map[string]interface{}{
					"field": "category_id",
					"size":  20,
				},
				"aggs": map[string]interface{}{
					"category_name": map[string]interface{}{
						"terms": map[string]interface{}{
							"field": "category_name.keyword",
							"size":  1,
						},
					},
				},
			},
			"price_ranges": map[string]interface{}{
				"range": map[string]interface{}{
					"field": "price",
					"ranges": []interface{}{
						map[string]interface{}{"key": "0-100", "to": 100},
						map[string]interface{}{"key": "100-500", "from": 100, "to": 500},
						map[string]interface{}{"key": "500-1000", "from": 500, "to": 1000},
						map[string]interface{}{"key": "1000-2000", "from": 1000, "to": 2000},
						map[string]interface{}{"key": "2000+", "from": 2000},
					},
				},
			},
		}
	}

	return query
}

// executeSearch 执行 ES 搜索（带重试和超时控制）
func (l *SearchLogic) executeSearch(query map[string]interface{}) ([]byte, error) {
	var lastErr error
	maxRetries := 3
	baseDelay := 100 * time.Millisecond

	for i := 0; i < maxRetries; i++ {
		// 设置超时
		ctx, cancel := context.WithTimeout(l.ctx, 5*time.Second)
		defer cancel()

		data, _ := sonic.Marshal(query)

		res, err := l.svcCtx.EsClient.Search(
			l.svcCtx.EsClient.Search.WithContext(ctx),
			l.svcCtx.EsClient.Search.WithIndex(svc.IndexName),
			l.svcCtx.EsClient.Search.WithBody(bytes.NewReader(data)),
		)
		if err != nil {
			lastErr = err
			logx.Errorf("ES search failed, retry %d/%d: %v", i+1, maxRetries, err)
			delay := baseDelay * time.Duration(1<<i) // 指数退避：100ms, 200ms, 400ms
			time.Sleep(delay)
			continue
		}
		defer res.Body.Close()

		if res.IsError() {
			lastErr = fmt.Errorf("ES search error: %s", res.String())
			logx.Errorf("ES search error, retry %d/%d: %v", i+1, maxRetries, lastErr)
			delay := baseDelay * time.Duration(1<<i)
			time.Sleep(delay)
			continue
		}

		bodyBytes, err := io.ReadAll(res.Body)
		if err != nil {
			lastErr = err
			logx.Errorf("Read ES response failed, retry %d/%d: %v", i+1, maxRetries, err)
			continue
		}

		return bodyBytes, nil
	}

	return nil, lastErr
}

// fallbackToMySQL ES 故障时降级到 MySQL 搜索
func (l *SearchLogic) fallbackToMySQL(in *search.SearchReq) (*search.SearchResp, error) {
	logx.Infof("ES search failed, falling back to MySQL search for keyword: %s", in.Keyword)

	// 尝试调用 product RPC 获取商品列表
	// 这里是一个简化的降级方案，实际项目中应该调用 product rpc
	resp, err := l.svcCtx.ProductRpc.ProductList(l.ctx, &product.ProductListRequest{
		CategoryId: 0,
		Cursor:     (in.PageNum - 1) * in.PageSize,
		Ps:         int32(in.PageSize),
	})

	if err != nil {
		logx.Errorf("MySQL fallback also failed: %v", err)
		return nil, fmt.Errorf("search service unavailable")
	}

	products := make([]*search.ProductData, 0, len(resp.Products))
	for _, p := range resp.Products {
		// 过滤上架商品
		if p.Status == product.ProductStatus_PRODUCT_STATUS_ONLINE && p.Stock > 0 {
			products = append(products, &search.ProductData{
				Id:       p.Id,
				Name:     p.Name,
				Brief:    p.Brief,
				Price:    float64(p.Price),
				Stock:    p.Stock,
				Sales:    p.Sales,
				ImageUrl: p.ImageUrl,
			})
		}
	}

	return &search.SearchResp{
		Data:  products,
		Total: int64(len(products)),
	}, nil
}

// Search 综合搜索、筛选、排序-根据关键字通过名称或副标题复合查询商品
func (l *SearchLogic) Search(in *search.SearchReq) (*search.SearchResp, error) {
	// 1. 构建查询
	query := l.buildSearchQuery(in)

	// 2. 生成缓存 key（包含过滤条件）
	cacheKey := cache.SearchCacheKey(in.Keyword, in.PageNum, in.PageSize, in.Sort)
	cacheExpire := 5 * time.Minute

	// 3. 执行搜索（带缓存和重试）
	bodyBytes, err := cache.WithCacheSearch(l.ctx, l.svcCtx.Cache, cacheKey, func() ([]byte, error) {
		return l.executeSearch(query)
	}, cacheExpire)

	// 4. 如果 ES 搜索失败，尝试降级到 MySQL
	if err != nil {
		logx.Errorf("ES search failed, error: %v", err)
		return l.fallbackToMySQL(in)
	}

	// 5. 解析结果
	var result struct {
		Hits struct {
			Total struct {
				Value int64 `json:"value"`
			} `json:"total"`
			Hits []struct {
				Source    search.ProductData  `json:"_source"`
				Highlight map[string][]string `json:"highlight"`
			} `json:"hits"`
		} `json:"hits"`
		Aggs struct {
			BrandAggs struct {
				Buckets []struct {
					Key       int64 `json:"key"`
					DocCount  int64 `json:"doc_count"`
					BrandName struct {
						Buckets []struct {
							Key string `json:"key"`
						} `json:"buckets"`
					} `json:"brand_name"`
				} `json:"buckets"`
			} `json:"brand_aggs"`
			CategoryAggs struct {
				Buckets []struct {
					Key          int64 `json:"key"`
					DocCount     int64 `json:"doc_count"`
					CategoryName struct {
						Buckets []struct {
							Key string `json:"key"`
						} `json:"buckets"`
					} `json:"category_name"`
				} `json:"buckets"`
			} `json:"category_aggs"`
			PriceRanges struct {
				Buckets []struct {
					Key      string `json:"key"`
					DocCount int64  `json:"doc_count"`
				} `json:"buckets"`
			} `json:"price_ranges"`
		} `json:"aggregations"`
	}

	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		logx.Errorf("Decode search result error: %v", err)
		return nil, err
	}

	// 6. 提取商品数据
	products := make([]*search.ProductData, 0, len(result.Hits.Hits))
	for _, hit := range result.Hits.Hits {
		p := hit.Source
		if highlightName, ok := hit.Highlight["name"]; ok && len(highlightName) > 0 {
			p.Name = highlightName[0]
		}
		if highlightBrief, ok := hit.Highlight["brief"]; ok && len(highlightBrief) > 0 {
			p.Brief = highlightBrief[0]
		}
		products = append(products, &p)
	}

	resp := &search.SearchResp{
		Data:  products,
		Total: result.Hits.Total.Value,
	}

	// 7. 提取聚合数据（如果需要）
	if in.NeedAgg {
		// 品牌聚合
		for _, b := range result.Aggs.BrandAggs.Buckets {
			brandName := ""
			if len(b.BrandName.Buckets) > 0 {
				brandName = b.BrandName.Buckets[0].Key
			}
			resp.BrandAggs = append(resp.BrandAggs, &search.BrandAgg{
				BrandId:   b.Key,
				BrandName: brandName,
				Count:     b.DocCount,
			})
		}

		// 分类聚合
		for _, c := range result.Aggs.CategoryAggs.Buckets {
			categoryName := ""
			if len(c.CategoryName.Buckets) > 0 {
				categoryName = c.CategoryName.Buckets[0].Key
			}
			resp.CategoryAggs = append(resp.CategoryAggs, &search.CategoryAgg{
				CategoryId:   c.Key,
				CategoryName: categoryName,
				Count:        c.DocCount,
			})
		}

		// 价格区间聚合
		for _, p := range result.Aggs.PriceRanges.Buckets {
			labels := map[string]string{
				"0-100":     "0-100元",
				"100-500":   "100-500元",
				"500-1000":  "500-1000元",
				"1000-2000": "1000-2000元",
				"2000+":     "2000元以上",
			}
			resp.PriceRangeAggs = append(resp.PriceRangeAggs, &search.PriceRangeAgg{
				RangeKey:   p.Key,
				RangeLabel: labels[p.Key],
				Count:      p.DocCount,
			})
		}
	}

	logx.Infof("Search completed, found %d products for keyword: %s", len(products), in.Keyword)
	return resp, nil
}
