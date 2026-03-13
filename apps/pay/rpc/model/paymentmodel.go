package model

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlc"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ PaymentModel = (*customPaymentModel)(nil)

type (
	Payment struct {
		PaymentId     string `db:"payment_id"`
		OrderId       string `db:"order_id"`
		UserId        int64  `db:"user_id"`
		Amount        int64  `db:"amount"`
		Status        int    `db:"status"`
		PaymentType   int    `db:"payment_type"`
		TransactionId string `db:"transaction_id"`
		CreateTime    int64  `db:"create_time"`
		PayTime       int64  `db:"pay_time"`
	}

	PaymentModel interface {
		Insert(ctx context.Context, payment *Payment) (sql.Result, error)
		Update(ctx context.Context, payment *Payment) error
		FindOneByPaymentId(ctx context.Context, paymentId string) (*Payment, error)
		FindOneByOrderId(ctx context.Context, orderId string) (*Payment, error)
	}

	customPaymentModel struct {
		sqlc.CachedConn
		table string
	}
)

func NewPaymentModel(conn sqlx.SqlConn, c cache.CacheConf) PaymentModel {
	return &customPaymentModel{
		CachedConn: sqlc.NewConn(conn, c),
		table:      "`payment`",
	}
}

func (m *customPaymentModel) Insert(ctx context.Context, payment *Payment) (sql.Result, error) {
	query := fmt.Sprintf("INSERT INTO %s (payment_id, order_id, user_id, amount, status, payment_type, transaction_id, create_time, pay_time) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)", m.table)
	return m.ExecCtx(ctx, func(ctx context.Context, conn sqlx.SqlConn) (sql.Result, error) {
		return conn.ExecCtx(ctx, query, payment.PaymentId, payment.OrderId, payment.UserId, payment.Amount,
			payment.Status, payment.PaymentType, payment.TransactionId, payment.CreateTime, payment.PayTime)
	})
}

func (m *customPaymentModel) Update(ctx context.Context, payment *Payment) error {
	paymentIdKey := fmt.Sprintf("cache:payment:paymentId:%s", payment.PaymentId)
	orderIdKey := fmt.Sprintf("cache:payment:orderId:%s", payment.OrderId)
	query := fmt.Sprintf("UPDATE %s SET status=?, transaction_id=?, pay_time=? WHERE payment_id=?", m.table)
	_, err := m.ExecCtx(ctx, func(ctx context.Context, conn sqlx.SqlConn) (sql.Result, error) {
		return conn.ExecCtx(ctx, query, payment.Status, payment.TransactionId, payment.PayTime, payment.PaymentId)
	})
	if err != nil {
		return err
	}
	return m.DelCacheCtx(ctx, paymentIdKey, orderIdKey)
}

func (m *customPaymentModel) FindOneByPaymentId(ctx context.Context, paymentId string) (*Payment, error) {
	paymentIdKey := fmt.Sprintf("cache:payment:paymentId:%s", paymentId)
	var result Payment
	err := m.QueryRowCtx(ctx, &result, paymentIdKey, func(ctx context.Context, conn sqlx.SqlConn, v any) error {
		query := fmt.Sprintf("SELECT payment_id, order_id, user_id, amount, status, payment_type, transaction_id, create_time, pay_time FROM %s WHERE payment_id = ?", m.table)
		return conn.QueryRowCtx(ctx, v, query, paymentId)
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (m *customPaymentModel) FindOneByOrderId(ctx context.Context, orderId string) (*Payment, error) {
	orderIdKey := fmt.Sprintf("cache:payment:orderId:%s", orderId)
	var result Payment
	err := m.QueryRowCtx(ctx, &result, orderIdKey, func(ctx context.Context, conn sqlx.SqlConn, v any) error {
		query := fmt.Sprintf("SELECT payment_id, order_id, user_id, amount, status, payment_type, transaction_id, create_time, pay_time FROM %s WHERE order_id = ? ORDER BY create_time DESC LIMIT 1", m.table)
		return conn.QueryRowCtx(ctx, v, query, orderId)
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}
