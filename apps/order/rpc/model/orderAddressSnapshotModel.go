package model

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ OrderAddressSnapshotModel = (*customOrderAddressSnapshotModel)(nil)

type (
	// OrderAddressSnapshotModel is an interface to be customized, add more methods here,
	// and implement the added methods in customOrderAddressSnapshotModel.
	OrderAddressSnapshotModel interface {
		orderAddressSnapshotModel
		TxInsert(ctx context.Context, tx *sql.Tx, data *OrderAddressSnapshot) (sql.Result, error)
	}

	customOrderAddressSnapshotModel struct {
		*defaultOrderAddressSnapshotModel
	}
)

// NewOrderAddressSnapshotModel returns a model for the database table.
func NewOrderAddressSnapshotModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) OrderAddressSnapshotModel {
	return &customOrderAddressSnapshotModel{
		defaultOrderAddressSnapshotModel: newOrderAddressSnapshotModel(conn, c, opts...),
	}
}

func (m customOrderAddressSnapshotModel) TxInsert(ctx context.Context, tx *sql.Tx, data *OrderAddressSnapshot) (sql.Result, error) {
	query := fmt.Sprintf(
		"insert into %s (order_id, user_id, name, phone, province, city, district, detail) "+
			"values (?, ?, ?, ?, ?, ?, ?, ?)",
		m.table,
	)

	return tx.ExecContext(
		ctx,
		query,
		data.OrderId,
		data.UserId,
		data.Name,
		data.Phone,
		data.Province,
		data.City,
		data.District,
		data.Detail,
	)
}
