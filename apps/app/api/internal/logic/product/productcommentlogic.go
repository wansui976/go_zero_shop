package product

import (
	"context"

	"github.com/wansui976/go_zero_shop/apps/app/api/internal/svc"
	"github.com/wansui976/go_zero_shop/apps/app/api/internal/types"
	replyclient "github.com/wansui976/go_zero_shop/apps/reply/rpc/replyclient"
	"github.com/zeromicro/go-zero/core/logx"
)

type ProductCommentLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 商品评论列表
func NewProductCommentLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProductCommentLogic {
	return &ProductCommentLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ProductCommentLogic) ProductComment(req *types.ProductCommentRequest) (resp *types.CommonResponse, err error) {
	rpcResp, err := l.svcCtx.ReplyRPC.Comments(l.ctx, &replyclient.CommentsRequest{
		Business: "product",
		TargetId: req.ProductID,
		Cursor:   req.Cursor,
		Ps:       int32(req.Ps),
	})
	if err != nil {
		return nil, err
	}

	data := &types.ProductCommentResponse{
		Comments:   buildProductComments(l.ctx, l.svcCtx.UserRPC, rpcResp.Comments),
		IsEnd:      rpcResp.IsEnd,
		LastCursor: rpcResp.LastCursor,
		TotalCount: rpcResp.Total,
	}

	return &types.CommonResponse{
		ResultCode: 200,
		Msg:        "success",
		Data:       data,
	}, nil
}
