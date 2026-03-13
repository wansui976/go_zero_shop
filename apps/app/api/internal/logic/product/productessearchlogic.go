package product

import (
	"context"

	"github.com/wansui976/go_zero_shop/apps/app/api/internal/svc"
	"github.com/wansui976/go_zero_shop/apps/app/api/internal/types"
	"github.com/wansui976/go_zero_shop/apps/search/rpc/searchclient"
	"github.com/zeromicro/go-zero/core/logx"
)

type ProductEsSearchLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 商品搜索
func NewProductEsSearchLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProductEsSearchLogic {
	return &ProductEsSearchLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ProductEsSearchLogic) ProductEsSearch(req *types.ProductEsSearchRequest) (resp *types.CommonResponse, err error) {
	var request searchclient.SearchReq
	request.Keyword = req.Keyword
	request.PageNum = req.Page_num
	request.PageSize = req.Page_size
	request.Sort = req.Sort
	response, err := l.svcCtx.SearchRPC.Search(l.ctx, &request)
	if err != nil {
		return nil, err
	}
	resp = &types.CommonResponse{
		ResultCode: 200,
		Msg:        "success",
		Data:       response.Data,
	}
	return resp, nil
}
