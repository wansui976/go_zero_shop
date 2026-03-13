package model

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ OrderItemsModel = (*customOrderItemsModel)(nil)

type (
	// OrderItemsModel is an interface to be customized, add more methods here,
	// and implement the added methods in customOrderItemsModel.
	OrderItemsModel interface {
		orderItemsModel
		TxInsert(ctx context.Context, tx *sql.Tx, data *OrderItems) error
		TxListByOrderId(ctx context.Context, tx *sql.Tx, orderId string) ([]*OrderItems, error)
	}

	customOrderItemsModel struct {
		*defaultOrderItemsModel
	}
)

// NewOrderItemsModel returns a model for the database table.
func NewOrderItemsModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) OrderItemsModel {
	return &customOrderItemsModel{
		defaultOrderItemsModel: newOrderItemsModel(conn, c, opts...),
	}
}

func (m *customOrderItemsModel) TxInsert(ctx context.Context, tx *sql.Tx, data *OrderItems) error {
	query := fmt.Sprintf(
		"INSERT INTO %s (order_id, product_id, quantity, unit_price, total_price, create_time, update_time) "+
			"VALUES (?, ?, ?, ?, ?, ?, ?)",
		m.table,
	)

	_, err := tx.ExecContext(
		ctx,
		query,
		data.OrderId,
		data.ProductId,
		data.Quantity,
		data.UnitPrice,
		data.TotalPrice,
		data.CreateTime,
		data.UpdateTime,
	)

	if err != nil {
		return fmt.Errorf("TxInsert OrderItems 失败: %w", err)
	}

	return nil
}
func (m *customOrderItemsModel) TxListByOrderId(
	ctx context.Context,
	tx *sql.Tx,
	orderId string,
) ([]*OrderItems, error) {

	query := `
	SELECT id, order_id, product_id, quantity
	FROM order_items
	WHERE order_id = ?
	`
	rows, err := tx.QueryContext(ctx, query, orderId)
	if err != nil {
		return nil, fmt.Errorf("TxListByOrderId 查询失败(orderId:%s):%w", orderId, err)
	}
	defer rows.Close()

	var items []*OrderItems
	for rows.Next() {
		var item OrderItems
		err := rows.Scan(
			&item.Id,
			&item.OrderId,
			&item.ProductId,
			&item.Quantity,
		)
		if err != nil {
			return nil, fmt.Errorf("TxListByOrderId 扫描失败(orderId:%s):%w", orderId, err)
		}
		items = append(items, &item)
	}

	return items, nil
}
