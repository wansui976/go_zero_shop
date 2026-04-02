package logic

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/bytedance/sonic"

	"github.com/wansui976/go_zero_shop/apps/product/rpc/product"
	"github.com/wansui976/go_zero_shop/apps/search/rpc/internal/svc"
	"github.com/wansui976/go_zero_shop/apps/search/rpc/search"
	"github.com/zeromicro/go-zero/core/logx"
)

type CreateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateLogic {
	return &CreateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 创建商品
func (l *CreateLogic) Create(in *search.CreateReq) (*search.CreateResp, error) {

	var productreq product.ProductAllListRequest
	res, err := l.svcCtx.ProductRpc.ProductAllList(l.ctx, &productreq)
	if err != nil {
		return &search.CreateResp{}, err
	}

	if len(res.Products) == 0 {
		return &search.CreateResp{}, nil
	}

	var bulkBuf bytes.Buffer
	productIDs := make([]int64, 0, len(res.Products))

	for _, p := range res.Products {
		if p.Name == "" {
			errMsg := fmt.Sprintf("invalid productData:id:%d,name=%s", p.Id, p.Name)
			logx.Errorf("%s", errMsg)
			return nil, fmt.Errorf("%s", errMsg)
		}
		productIDs = append(productIDs, p.Id)

		// 1. 写入删除操作（清理旧数据）
		deleteMeta := map[string]map[string]string{
			"delete": {"_index": svc.IndexName, "_id": strconv.FormatInt(p.Id, 10)},
		}
		deleteLine, err := sonic.Marshal(deleteMeta)
		if err != nil {
			errMsg := fmt.Sprintf("marshal deleteMeta failed for product %d: %v", p.Id, err)
			logx.Error(errMsg)
			return nil, fmt.Errorf("%s", errMsg)
		}

		bulkBuf.Write(deleteLine)
		bulkBuf.WriteByte('\n')
		// 2. 写入创建操作（新增/更新数据）
		meta := map[string]map[string]string{
			"index": {"_index": svc.IndexName, "_id": strconv.FormatInt(p.Id, 10)},
		}
		metaLine, err := sonic.Marshal(meta)
		if err != nil {
			errMsg := fmt.Sprintf("marshal index meta failed for product %d: %v", p.Id, err)
			logx.Error(errMsg)
			return nil, fmt.Errorf("%s", errMsg)
		}
		bulkBuf.Write(metaLine)
		bulkBuf.WriteByte('\n')

		data, err := sonic.Marshal(p)
		if err != nil {
			errMsg := fmt.Sprintf("marshal product data failed for product %d: %v", p.Id, err)
			logx.Error(errMsg)
			return nil, fmt.Errorf("%s", errMsg)
		}
		bulkBuf.Write(data)
		bulkBuf.WriteByte('\n')

	}

	ctx, _ := context.WithTimeout(l.ctx, 10*time.Second)

	_, err = l.svcCtx.EsClient.Bulk(bytes.NewReader(bulkBuf.Bytes()), l.svcCtx.EsClient.Bulk.WithContext(ctx))

	if err != nil {
		errMsg := fmt.Sprintf("bulk delete+create error for product IDs %v: %v", productIDs, err)
		logx.Error(errMsg)
		return nil, fmt.Errorf("%s", errMsg)
	}

	logx.Infof("商品es索引更新成功，商品ID：%v", productIDs)
	return &search.CreateResp{}, nil
}
