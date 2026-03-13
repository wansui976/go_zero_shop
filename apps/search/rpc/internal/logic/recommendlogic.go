package logic

import (
	"bytes"
	"context"
	"encoding/json"

	"github.com/bytedance/sonic"
	"github.com/wansui976/go_zero_shop/apps/search/rpc/internal/svc"
	"github.com/wansui976/go_zero_shop/apps/search/rpc/search"

	"github.com/zeromicro/go-zero/core/logx"
)

type RecommendLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRecommendLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RecommendLogic {
	return &RecommendLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 根据商品ID或关键词推荐商品（More Like This 算法）
func (l *RecommendLogic) Recommend(in *search.RecommendReq) (*search.SearchResp, error) {
	// 构建 More Like This 查询
	mltQuery := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []interface{}{
					map[string]interface{}{
						"more_like_this": map[string]interface{}{
							// 字段权重：name 最重要，keywords 次之
							"fields":               []string{"name^3", "keywords^2", "brief^1.5", "detail_title^1.2", "detail_desc^1"},
							"like":                 in.Keyword,
							"min_term_freq":        1,
							"min_doc_freq":         1,
							"max_query_terms":      25,
							"minimum_should_match": "30%",
						},
					},
				},
			},
		},
		"from": (in.PageNum - 1) * in.PageSize,
		"size": in.PageSize,
	}

	// 添加筛选条件（可选）
	if in.CategoryId > 0 {
		mltQuery["query"].(map[string]interface{})["bool"].(map[string]interface{})["filter"] = []interface{}{
			map[string]interface{}{
				"term": map[string]interface{}{
					"category_id": in.CategoryId,
				},
			},
		}
	}

	if in.BrandId > 0 {
		filter := mltQuery["query"].(map[string]interface{})["bool"].(map[string]interface{})["filter"]
		if filter == nil {
			filter = []interface{}{}
		}
		filter = append(filter.([]interface{}), map[string]interface{}{
			"term": map[string]interface{}{
				"brand_id": in.BrandId,
			},
		})
		mltQuery["query"].(map[string]interface{})["bool"].(map[string]interface{})["filter"] = filter
	}

	// 添加排序（按相关度或推荐排序）
	if in.Sort == "recommend" || in.Sort == "" {
		mltQuery["sort"] = []interface{}{
			map[string]interface{}{"recommend_status_sort": map[string]string{"order": "desc"}},
			map[string]interface{}{"sales": map[string]string{"order": "desc"}},
		}
	}

	// 序列化查询
	data, _ := sonic.Marshal(mltQuery)

	// 执行搜索
	res, err := l.svcCtx.EsClient.Search(
		l.svcCtx.EsClient.Search.WithContext(l.ctx),
		l.svcCtx.EsClient.Search.WithIndex(svc.IndexName),
		l.svcCtx.EsClient.Search.WithBody(bytes.NewReader(data)),
	)
	if err != nil {
		logx.Errorf("Recommend search error: %v", err)
		return nil, err
	}
	defer res.Body.Close()

	// 解析结果
	var result struct {
		Hits struct {
			Total struct {
				Value int64 `json:"value"`
			} `json:"total"`
			Hits []struct {
				Source search.ProductData `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}

	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		logx.Errorf("Decode recommend result error: %v", err)
		return nil, err
	}

	// 组装返回结果
	products := make([]*search.ProductData, 0, len(result.Hits.Hits))
	for _, hit := range result.Hits.Hits {
		products = append(products, &hit.Source)
	}

	logx.Infof("Recommend search completed, found %d products", len(products))
	return &search.SearchResp{
		Data:  products,
		Total: result.Hits.Total.Value,
	}, nil
}
