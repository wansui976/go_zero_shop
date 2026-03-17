package logic

import (
	"context"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/wansui976/go_zero_shop/apps/reply/rpc/internal/svc"
	"github.com/wansui976/go_zero_shop/apps/reply/rpc/model"
	"github.com/wansui976/go_zero_shop/apps/reply/rpc/reply"
	"github.com/wansui976/go_zero_shop/pkg/xerr"
	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateCommentLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateCommentLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateCommentLogic {
	return &UpdateCommentLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdateCommentLogic) UpdateComment(in *reply.UpdateCommentRequest) (*reply.UpdateCommentResponse, error) {
	if in.Id <= 0 {
		return nil, errors.Wrapf(xerr.NewErrCode(xerr.ReuqestParamError), "评论ID非法: %d", in.Id)
	}

	content := strings.TrimSpace(in.Content)
	image := model.NormalizeImages(strings.TrimSpace(in.Image))
	if content == "" && image == "" {
		return nil, errors.Wrap(xerr.NewErrCode(xerr.ReuqestParamError), "评论内容和图片不能同时为空")
	}

	if _, err := l.svcCtx.CommentModel.FindActiveOne(l.ctx, in.Id); err != nil {
		if err == model.ErrNotFound {
			return nil, errors.Wrapf(xerr.NewErrCode(xerr.DataNoExistError), "评论不存在: id=%d", in.Id)
		}
		return nil, errors.Wrapf(xerr.NewErrCode(xerr.DbError), "查询评论失败: %v", err)
	}

	if err := l.svcCtx.CommentModel.UpdateContent(l.ctx, in.Id, content, image, time.Now().UnixMilli()); err != nil {
		if err == model.ErrNotFound {
			return nil, errors.Wrapf(xerr.NewErrCode(xerr.DataNoExistError), "评论不存在: id=%d", in.Id)
		}
		return nil, errors.Wrapf(xerr.NewErrCode(xerr.DbError), "更新评论失败: %v", err)
	}

	return &reply.UpdateCommentResponse{}, nil
}
