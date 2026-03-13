package model

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlc"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ RefundModel = (*customRefundModel)(nil)

type (
	Refund struct {
		RefundId     string `db:"refund_id"`
		OrderId      string `db:"order_id"`
		PaymentId    string `db:"payment_id"`
		RefundAmount int64  `db:"refund_amount"`
		Status       int    `db:"status"`
		Reason       string `db:"reason"`
		CreateTime   int64  `db:"create_time"`
		RefundTime   int64  `db:"refund_time"`
	}

	RefundModel interface {
		Insert(ctx context.Context, refund *Refund) (sql.Result, error)
		Update(ctx context.Context, refund *Refund) error
		FindOneByRefundId(ctx context.Context, refundId string) (*Refund, error)
		FindOneByOrderId(ctx context.Context, orderId string) (*Refund, error)
		SumRefundAmountByPaymentId(ctx context.Context, paymentId string) (int64, error)
	}

	customRefundModel struct {
		sqlc.CachedConn
		table string
	}
)

func NewRefundModel(conn sqlx.SqlConn, c cache.CacheConf) RefundModel {
	return &customRefundModel{
		CachedConn: sqlc.NewConn(conn, c),
		table:      "`refund`",
	}
}

func (m *customRefundModel) Insert(ctx context.Context, refund *Refund) (sql.Result, error) {
	query := fmt.Sprintf("INSERT INTO %s (refund_id, order_id, payment_id, refund_amount, status, reason, create_time, refund_time) VALUES (?, ?, ?, ?, ?, ?, ?, ?)", m.table)
	return m.ExecCtx(ctx, func(ctx context.Context, conn sqlx.SqlConn) (sql.Result, error) {
		return conn.ExecCtx(ctx, query, refund.RefundId, refund.OrderId, refund.PaymentId, refund.RefundAmount,
			refund.Status, refund.Reason, refund.CreateTime, refund.RefundTime)
	})
}

func (m *customRefundModel) Update(ctx context.Context, refund *Refund) error {
	refundIdKey := fmt.Sprintf("cache:refund:refundId:%s", refund.RefundId)
	orderIdKey := fmt.Sprintf("cache:refund:orderId:%s", refund.OrderId)
	query := fmt.Sprintf("UPDATE %s SET status=?, refund_time=? WHERE refund_id=?", m.table)
	_, err := m.ExecCtx(ctx, func(ctx context.Context, conn sqlx.SqlConn) (sql.Result, error) {
		return conn.ExecCtx(ctx, query, refund.Status, refund.RefundTime, refund.RefundId)
	})
	if err != nil {
		return err
	}
	return m.DelCacheCtx(ctx, refundIdKey, orderIdKey)
}

func (m *customRefundModel) FindOneByRefundId(ctx context.Context, refundId string) (*Refund, error) {
	refundIdKey := fmt.Sprintf("cache:refund:refundId:%s", refundId)
	var result Refund
	err := m.QueryRowCtx(ctx, &result, refundIdKey, func(ctx context.Context, conn sqlx.SqlConn, v any) error {
		query := fmt.Sprintf("SELECT refund_id, order_id, payment_id, refund_amount, status, reason, create_time, refund_time FROM %s WHERE refund_id = ?", m.table)
		return conn.QueryRowCtx(ctx, v, query, refundId)
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (m *customRefundModel) FindOneByOrderId(ctx context.Context, orderId string) (*Refund, error) {
	orderIdKey := fmt.Sprintf("cache:refund:orderId:%s", orderId)
	var result Refund
	err := m.QueryRowCtx(ctx, &result, orderIdKey, func(ctx context.Context, conn sqlx.SqlConn, v any) error {
		query := fmt.Sprintf("SELECT refund_id, order_id, payment_id, refund_amount, status, reason, create_time, refund_time FROM %s WHERE order_id = ? ORDER BY create_time DESC LIMIT 1", m.table)
		return conn.QueryRowCtx(ctx, v, query, orderId)
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (m *customRefundModel) SumRefundAmountByPaymentId(ctx context.Context, paymentId string) (int64, error) {
	var total sql.NullInt64
	query := fmt.Sprintf(
		"SELECT COALESCE(SUM(refund_amount), 0) FROM %s WHERE payment_id = ? AND status IN (?, ?)",
		m.table,
	)
	if err := m.QueryRowNoCacheCtx(ctx, &total, query, paymentId, 0, 1); err != nil {
		return 0, err
	}
	if !total.Valid {
		return 0, nil
	}
	return total.Int64, nil
}
