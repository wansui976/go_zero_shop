package product

import (
	"context"

	"github.com/wansui976/go_zero_shop/apps/app/api/internal/svc"
	"github.com/wansui976/go_zero_shop/apps/app/api/internal/types"
	"github.com/wansui976/go_zero_shop/apps/search/rpc/searchclient"

	"github.com/zeromicro/go-zero/core/logx"
)

type ProductESLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 商品索引
func NewProductESLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProductESLogic {
	return &ProductESLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ProductESLogic) ProductES(req *types.ProductEsRequest) (resp *types.CommonResponse, err error) {
	var request searchclient.CreateReq
	_, err = l.svcCtx.SearchRPC.Create(l.ctx, &request)
	if err != nil {
		return nil, err
	}
	resp = &types.CommonResponse{
		ResultCode: 200,
		Msg:        "success",
		Data:       "",
	}
	return resp, nil
}
