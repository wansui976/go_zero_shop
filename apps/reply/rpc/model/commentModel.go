package model

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ CommentModel = (*customCommentModel)(nil)

type (
	CommentModel interface {
		commentModel
		Create(ctx context.Context, data *Comment) (sql.Result, error)
		FindActiveOne(ctx context.Context, id int64) (*Comment, error)
		FindListByTarget(ctx context.Context, business string, targetID, cursor int64, ps int64) ([]*Comment, error)
		CountByTarget(ctx context.Context, business string, targetID int64) (int64, error)
		UpdateContent(ctx context.Context, id int64, content, image string, updateTime int64) error
		SoftDelete(ctx context.Context, id int64, updateTime int64) error
	}

	customCommentModel struct {
		*defaultCommentModel
	}
)

func NewCommentModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) CommentModel {
	return &customCommentModel{
		defaultCommentModel: newCommentModel(conn, c, opts...),
	}
}

func (m *customCommentModel) Create(ctx context.Context, data *Comment) (sql.Result, error) {
	commentIDKey := fmt.Sprintf("%s%v", cacheCommentIdPrefix, data.Id)
	query := fmt.Sprintf("insert into %s (id, business, target_id, reply_user_id, be_reply_user_id, parent_id, content, image, status, create_time, update_time) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)", m.table)
	return m.ExecCtx(ctx, func(ctx context.Context, conn sqlx.SqlConn) (sql.Result, error) {
		return conn.ExecCtx(ctx, query,
			data.Id,
			data.Business,
			data.TargetId,
			data.ReplyUserId,
			data.BeReplyUserId,
			data.ParentId,
			data.Content,
			data.Image,
			data.Status,
			data.CreateTime,
			data.UpdateTime,
		)
	}, commentIDKey)
}

func (m *customCommentModel) FindActiveOne(ctx context.Context, id int64) (*Comment, error) {
	var resp Comment
	query := fmt.Sprintf("select %s from %s where id = ? and status = 1 limit 1", commentRows, m.table)
	if err := m.QueryRowNoCacheCtx(ctx, &resp, query, id); err != nil {
		if err == sqlx.ErrNotFound {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &resp, nil
}

func (m *customCommentModel) FindListByTarget(ctx context.Context, business string, targetID, cursor int64, ps int64) ([]*Comment, error) {
	var (
		resp  []*Comment
		args  = []interface{}{business, targetID, 1}
		query = fmt.Sprintf("select %s from %s where business = ? and target_id = ? and status = ?", commentRows, m.table)
	)

	if cursor > 0 {
		query += " and id < ?"
		args = append(args, cursor)
	}

	query += " order by id desc limit ?"
	args = append(args, ps)

	if err := m.QueryRowsNoCacheCtx(ctx, &resp, query, args...); err != nil {
		if err == sqlx.ErrNotFound {
			return []*Comment{}, nil
		}
		return nil, err
	}

	if resp == nil {
		return []*Comment{}, nil
	}
	return resp, nil
}

func (m *customCommentModel) CountByTarget(ctx context.Context, business string, targetID int64) (int64, error) {
	var total int64
	query := fmt.Sprintf("select count(1) from %s where business = ? and target_id = ? and status = 1", m.table)
	if err := m.QueryRowNoCacheCtx(ctx, &total, query, business, targetID); err != nil {
		if err == sqlx.ErrNotFound {
			return 0, nil
		}
		return 0, err
	}
	return total, nil
}

func (m *customCommentModel) UpdateContent(ctx context.Context, id int64, content, image string, updateTime int64) error {
	commentIDKey := fmt.Sprintf("%s%v", cacheCommentIdPrefix, id)
	query := fmt.Sprintf("update %s set content = ?, image = ?, update_time = ? where id = ? and status = 1", m.table)
	res, err := m.ExecCtx(ctx, func(ctx context.Context, conn sqlx.SqlConn) (sql.Result, error) {
		return conn.ExecCtx(ctx, query, content, image, updateTime, id)
	}, commentIDKey)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err == nil && rows == 0 {
		return ErrNotFound
	}
	return nil
}

func (m *customCommentModel) SoftDelete(ctx context.Context, id int64, updateTime int64) error {
	commentIDKey := fmt.Sprintf("%s%v", cacheCommentIdPrefix, id)
	query := fmt.Sprintf("update %s set status = 0, update_time = ? where id = ? and status = 1", m.table)
	res, err := m.ExecCtx(ctx, func(ctx context.Context, conn sqlx.SqlConn) (sql.Result, error) {
		return conn.ExecCtx(ctx, query, updateTime, id)
	}, commentIDKey)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err == nil && rows == 0 {
		return ErrNotFound
	}
	return nil
}

func NormalizeImages(image string) string {
	parts := strings.Split(image, ",")
	clean := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			clean = append(clean, part)
		}
	}
	return strings.Join(clean, ",")
}
