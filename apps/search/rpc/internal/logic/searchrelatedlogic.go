package logic

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"github.com/wansui976/go_zero_shop/apps/search/rpc/internal/svc"
	"github.com/wansui976/go_zero_shop/apps/search/rpc/search"

	"github.com/zeromicro/go-zero/core/logx"
)

type SearchRelatedLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSearchRelatedLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SearchRelatedLogic {
	return &SearchRelatedLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 获取搜索的相关品牌、分类及筛选属性
func (l *SearchRelatedLogic) SearchRelated(in *search.SearchRelatedReq) (*search.SearchRelatedResp, error) {
	// 构建聚合查询
	aggQuery := map[string]interface{}{
		"size": 0, // 不返回具体文档，只返回聚合结果
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []interface{}{
					map[string]interface{}{
						"multi_match": map[string]interface{}{
							"query":  in.Keyword,
							// 字段权重配置
							"fields": []string{"name^3", "keywords^2", "brief^1.5", "detail_title^1.2", "detail_desc^1"},
							"type":  "best_fields",
						},
					},
				},
			},
		},
		"aggs": map[string]interface{}{
			// 聚合品牌名称
			"brand_names": map[string]interface{}{
				"terms": map[string]interface{}{
					"field": "brand_name.keyword",
					"size":  20,
				},
			},
			// 聚合分类名称
			"category_names": map[string]interface{}{
				"terms": map[string]interface{}{
					"field": "category_name.keyword",
					"size":  20,
				},
			},
		},
	}

	// 序列化查询
	data, _ := json.Marshal(aggQuery)

	// 执行搜索
	res, err := l.svcCtx.EsClient.Search(
		l.svcCtx.EsClient.Search.WithContext(l.ctx),
		l.svcCtx.EsClient.Search.WithIndex(svc.IndexName),
		l.svcCtx.EsClient.Search.WithBody(bytes.NewReader(data)),
	)
	if err != nil {
		logx.Errorf("SearchRelated search error: %v", err)
		return nil, err
	}
	defer res.Body.Close()

	// 解析聚合结果
	var result struct {
		Aggregations struct {
			BrandNames struct {
				Buckets []struct {
					Key string `json:"key"`
				} `json:"buckets"`
			} `json:"brand_names"`
			CategoryNames struct {
				Buckets []struct {
					Key string `json:"key"`
				} `json:"buckets"`
			} `json:"category_names"`
		} `json:"aggregations"`
	}

	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		logx.Errorf("Decode aggregations error: %v", err)
		return nil, err
	}

	// 组装返回结果
	brandNames := make([]string, 0, len(result.Aggregations.BrandNames.Buckets))
	for _, b := range result.Aggregations.BrandNames.Buckets {
		if b.Key != "" {
			brandNames = append(brandNames, b.Key)
		}
	}

	categoryNames := make([]string, 0, len(result.Aggregations.CategoryNames.Buckets))
	for _, c := range result.Aggregations.CategoryNames.Buckets {
		if c.Key != "" {
			categoryNames = append(categoryNames, c.Key)
		}
	}

	// 聚合筛选项（这里返回示例属性，实际可扩展）
	productAttrs := make([]*search.ProductAttr, 0)

	logx.Infof("SearchRelated completed, found %d brands, %d categories", len(brandNames), len(categoryNames))
	return &search.SearchRelatedResp{
		BrandNames:   brandNames,
		CategoryNames: categoryNames,
		ProductAttrs: productAttrs,
	}, nil
}

// CleanKeyword 清理关键词（去除特殊字符）
func CleanKeyword(keyword string) string {
	// 去除 ES 特殊字符
	keyword = strings.ReplaceAll(keyword, "+", "")
	keyword = strings.ReplaceAll(keyword, "-", "")
	keyword = strings.ReplaceAll(keyword, "*", "")
	keyword = strings.ReplaceAll(keyword, "/", "")
	keyword = strings.ReplaceAll(keyword, "(", "")
	keyword = strings.ReplaceAll(keyword, ")", "")
	return keyword
}
