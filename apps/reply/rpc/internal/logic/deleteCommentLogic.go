package logic

import (
	"context"
	"time"

	"github.com/pkg/errors"
	"github.com/wansui976/go_zero_shop/apps/reply/rpc/internal/svc"
	"github.com/wansui976/go_zero_shop/apps/reply/rpc/model"
	"github.com/wansui976/go_zero_shop/apps/reply/rpc/reply"
	"github.com/wansui976/go_zero_shop/pkg/xerr"
	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteCommentLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDeleteCommentLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteCommentLogic {
	return &DeleteCommentLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *DeleteCommentLogic) DeleteComment(in *reply.DeleteCommentRequest) (*reply.DeleteCommentResponse, error) {
	if in.Id <= 0 {
		return nil, errors.Wrapf(xerr.NewErrCode(xerr.ReuqestParamError), "评论ID非法: %d", in.Id)
	}

	if err := l.svcCtx.CommentModel.SoftDelete(l.ctx, in.Id, time.Now().UnixMilli()); err != nil {
		if err == model.ErrNotFound {
			return nil, errors.Wrapf(xerr.NewErrCode(xerr.DataNoExistError), "评论不存在: id=%d", in.Id)
		}
		return nil, errors.Wrapf(xerr.NewErrCode(xerr.DbError), "删除评论失败: %v", err)
	}

	return &reply.DeleteCommentResponse{}, nil
}
