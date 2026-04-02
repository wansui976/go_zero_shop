package logic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"
	"github.com/wansui976/go_zero_shop/apps/search/rpc/internal/svc"
	"github.com/wansui976/go_zero_shop/apps/search/rpc/search"

	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteLogic {
	return &DeleteLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 根据id集合删除商品
func (l *DeleteLogic) Delete(in *search.DeleteReq) (*search.DeleteResp, error) {
	// 安全检查：防止误删全量数据
	if in.Ids == nil || len(in.Ids) == 0 {
		logx.Infof("Delete request received with empty IDs, skipping deletion to prevent data loss")
		return &search.DeleteResp{}, nil
	}

	var buf bytes.Buffer
	productIDs := make([]int64, 0, len(in.Ids))
	for _, id := range in.Ids {
		productIDs = append(productIDs, id)
		meta := map[string]map[string]string{
			"delete": {"_index": svc.IndexName, "_id": strconv.FormatInt(id, 10)},
		}
		line, err := json.Marshal(meta)
		if err != nil {
			errMsg := fmt.Sprintf("marshal delete meta failed for product ID %d: %v", id, err)
			logx.Error(errMsg)
			return nil, fmt.Errorf("%s", errMsg)
		}
		buf.Write(line)
		buf.WriteByte('\n')
	}

	// 超时控制
	ctx, cancel := context.WithTimeout(l.ctx, 10*time.Second)
	defer cancel()

	_, err := l.svcCtx.EsClient.Bulk(bytes.NewReader(buf.Bytes()), l.svcCtx.EsClient.Bulk.WithContext(ctx))
	if err != nil {
		errMsg := fmt.Sprintf("bulk delete error for product IDs %v: %v", productIDs, err)
		logx.Error(errMsg)
		return nil, fmt.Errorf("%s", errMsg)
	}

	logx.Infof("删除商品es索引成功，商品ID：%v", productIDs)
	return &search.DeleteResp{}, nil
}
