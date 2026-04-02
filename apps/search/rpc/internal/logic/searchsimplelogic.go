package logic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"github.com/wansui976/go_zero_shop/apps/search/rpc/internal/svc"
	"github.com/wansui976/go_zero_shop/apps/search/rpc/search"

	//"github.com/wansui976/go_zero_shop/apps/search/internal/svc"
	//"github.com/wansui976/go_zero_shop/apps/search/search"
	"github.com/zeromicro/go-zero/core/logx"
)

type SearchSimpleLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSearchSimpleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SearchSimpleLogic {
	return &SearchSimpleLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// SearchSimple 简单搜索-根据关键字通过名称或副标题查询商品
func (l *SearchSimpleLogic) SearchSimple(in *search.SearchSimpleReq) (*search.SearchResp, error) {
	// 关键词空值判断
	if strings.TrimSpace(in.Keyword) == "" {
		return &search.SearchResp{Data: []*search.ProductData{}, Total: 0}, nil
	}

	query := map[string]interface{}{
		"query": map[string]interface{}{
			"multi_match": map[string]interface{}{
				"query":  in.Keyword,
				"fields": []string{"name", "brief"},
			},
		},
		"from": (in.PageNum - 1) * in.PageSize,
		"size": in.PageSize,
		"highlight": map[string]interface{}{
			"fields": map[string]interface{}{
				"name":  map[string]interface{}{},
				"brief": map[string]interface{}{},
			},
		},
	}

	// 序列化并校验错误
	data, err := json.Marshal(query)
	if err != nil {
		errMsg := fmt.Sprintf("marshal simple search query failed: %v", err)
		logx.Error(errMsg)
		return nil, fmt.Errorf("%s", errMsg)
	}

	// 超时控制（10秒）
	ctx, cancel := context.WithTimeout(l.ctx, 10*time.Second)
	defer cancel()

	// 发起 ES 搜索请求
	res, err := l.svcCtx.EsClient.Search(
		l.svcCtx.EsClient.Search.WithContext(ctx),
		l.svcCtx.EsClient.Search.WithIndex(svc.IndexName),
		l.svcCtx.EsClient.Search.WithBody(bytes.NewReader(data)),
	)
	if err != nil {
		errMsg := fmt.Sprintf("es simple search error: %v", err)
		logx.Error(errMsg)
		return nil, fmt.Errorf("%s", errMsg)
	}

	// 解析 ES 返回结果
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
	}
	err = json.NewDecoder(res.Body).Decode(&result)
	if err != nil {
		errMsg := fmt.Sprintf("decode es simple search result failed: %v", err)
		logx.Error(errMsg)
		return nil, fmt.Errorf("%s", errMsg)
	}

	// 组装商品列表（处理高亮）
	products := make([]*search.ProductData, 0, len(result.Hits.Hits))
	for _, hit := range result.Hits.Hits {
		p := hit.Source
		// 替换名称高亮
		if highlightName, ok := hit.Highlight["name"]; ok && len(highlightName) > 0 {
			p.Name = highlightName[0]
		}
		// 替换副标题高亮
		if highlightBrief, ok := hit.Highlight["brief"]; ok && len(highlightBrief) > 0 {
			p.Brief = highlightBrief[0]
		}
		products = append(products, &p)
	}

	// 返回结果（包含总条数和商品列表）
	return &search.SearchResp{
		Data:  products,
		Total: result.Hits.Total.Value,
	}, nil
}
