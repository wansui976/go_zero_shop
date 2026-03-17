package logic

import (
	"context"
	"strings"

	"github.com/pkg/errors"
	"github.com/wansui976/go_zero_shop/apps/reply/rpc/internal/svc"
	"github.com/wansui976/go_zero_shop/apps/reply/rpc/reply"
	"github.com/wansui976/go_zero_shop/pkg/xerr"
	"github.com/zeromicro/go-zero/core/logx"
)

type CommentsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCommentsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CommentsLogic {
	return &CommentsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CommentsLogic) Comments(in *reply.CommentsRequest) (*reply.CommentsResponse, error) {
	business := strings.TrimSpace(in.Business)
	if business == "" || in.TargetId <= 0 {
		return nil, errors.Wrapf(xerr.NewErrCode(xerr.ReuqestParamError), "查询评论参数非法: business=%q target_id=%d", business, in.TargetId)
	}

	ps := int64(in.Ps)
	if ps <= 0 {
		ps = 20
	}
	if ps > 50 {
		ps = 50
	}

	list, err := l.svcCtx.CommentModel.FindListByTarget(l.ctx, business, in.TargetId, in.Cursor, ps)
	if err != nil {
		return nil, errors.Wrapf(xerr.NewErrCode(xerr.DbError), "查询评论列表失败: %v", err)
	}

	total, err := l.svcCtx.CommentModel.CountByTarget(l.ctx, business, in.TargetId)
	if err != nil {
		return nil, errors.Wrapf(xerr.NewErrCode(xerr.DbError), "统计评论数量失败: %v", err)
	}

	items := make([]*reply.CommentItem, 0, len(list))
	var lastCursor int64
	for _, item := range list {
		items = append(items, &reply.CommentItem{
			Id:            item.Id,
			Business:      item.Business,
			TargetId:      item.TargetId,
			ReplyUserId:   item.ReplyUserId,
			BeReplyUserId: item.BeReplyUserId,
			ParentId:      item.ParentId,
			Content:       item.Content,
			Image:         item.Image,
			CreateTime:    item.CreateTime,
			UpdateTime:    item.UpdateTime,
		})
		lastCursor = item.Id
	}

	return &reply.CommentsResponse{
		Comments:   items,
		IsEnd:      int64(len(list)) < ps,
		LastCursor: lastCursor,
		Total:      total,
	}, nil
}
