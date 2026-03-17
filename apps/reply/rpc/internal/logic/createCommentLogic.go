package logic

import (
	"context"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/wansui976/go_zero_shop/apps/reply/rpc/internal/svc"
	"github.com/wansui976/go_zero_shop/apps/reply/rpc/model"
	"github.com/wansui976/go_zero_shop/apps/reply/rpc/reply"
	"github.com/wansui976/go_zero_shop/pkg/snowflake"
	"github.com/wansui976/go_zero_shop/pkg/xerr"
	"github.com/zeromicro/go-zero/core/logx"
)

type CreateCommentLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateCommentLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateCommentLogic {
	return &CreateCommentLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CreateCommentLogic) CreateComment(in *reply.CreateCommentRequest) (*reply.CreateCommentResponse, error) {
	business := strings.TrimSpace(in.Business)
	content := strings.TrimSpace(in.Content)
	image := model.NormalizeImages(strings.TrimSpace(in.Image))

	if business == "" || in.TargetId <= 0 || in.ReplyUserId <= 0 {
		return nil, errors.Wrapf(xerr.NewErrCode(xerr.ReuqestParamError), "评论参数非法: business=%q target_id=%d reply_user_id=%d", business, in.TargetId, in.ReplyUserId)
	}
	if content == "" && image == "" {
		return nil, errors.Wrap(xerr.NewErrCode(xerr.ReuqestParamError), "评论内容和图片不能同时为空")
	}
	if in.BeReplyUserId < 0 || in.ParentId < 0 {
		return nil, errors.Wrapf(xerr.NewErrCode(xerr.ReuqestParamError), "回复参数非法: be_reply_user_id=%d parent_id=%d", in.BeReplyUserId, in.ParentId)
	}

	if in.ParentId > 0 {
		parent, err := l.svcCtx.CommentModel.FindActiveOne(l.ctx, in.ParentId)
		if err != nil {
			if err == model.ErrNotFound {
				return nil, errors.Wrapf(xerr.NewErrCode(xerr.DataNoExistError), "父评论不存在: parent_id=%d", in.ParentId)
			}
			return nil, errors.Wrapf(xerr.NewErrCode(xerr.DbError), "查询父评论失败: %v", err)
		}
		if parent.Business != business || parent.TargetId != in.TargetId {
			return nil, errors.Wrapf(xerr.NewErrCode(xerr.ReuqestParamError), "父评论与当前业务对象不匹配: parent_id=%d", in.ParentId)
		}
	}

	id, err := snowflake.GenIDInt()
	if err != nil {
		return nil, errors.Wrapf(xerr.NewErrCode(xerr.SnowflakeError), "生成评论ID失败: %v", err)
	}

	now := time.Now().UnixMilli()
	comment := &model.Comment{
		Id:            id,
		Business:      business,
		TargetId:      in.TargetId,
		ReplyUserId:   in.ReplyUserId,
		BeReplyUserId: in.BeReplyUserId,
		ParentId:      in.ParentId,
		Content:       content,
		Image:         image,
		Status:        1,
		CreateTime:    now,
		UpdateTime:    now,
	}

	if _, err = l.svcCtx.CommentModel.Create(l.ctx, comment); err != nil {
		return nil, errors.Wrapf(xerr.NewErrCode(xerr.DbError), "创建评论失败: %v", err)
	}

	return &reply.CreateCommentResponse{Id: id}, nil
}
