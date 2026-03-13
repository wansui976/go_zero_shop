package model

import (
	"context"
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ CartModel = (*customCartModel)(nil)

type (
	// CartModel is an interface to be customized, add more methods here,
	// and implement the added methods in customCartModel.
	CartModel interface {
		cartModel
		FindOneByUserAndProduct(ctx context.Context, userId, productId int64) (*Cart, error)
		FindByUserId(ctx context.Context, userId int64) ([]*Cart, error)
	}

	customCartModel struct {
		*defaultCartModel
	}
)

// NewCartModel returns a model for the database table.
func NewCartModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) CartModel {
	return &customCartModel{
		defaultCartModel: newCartModel(conn, c, opts...),
	}
}

func (m customCartModel) FindOneByUserAndProduct(ctx context.Context, userId, productId int64) (*Cart, error) {
	var cart Cart
	err := m.QueryRowNoCacheCtx(ctx, &cart, "SELECT * FROM cart WHERE user_id=? AND product_id=? AND delete_status=0", userId, productId)
	if err != nil {
		return nil, err
	}
	return &cart, nil
}
func (m *customCartModel) FindByUserId(ctx context.Context, userId int64) ([]*Cart, error) {
	var carts []*Cart
	query := "SELECT * FROM cart WHERE user_id = ? AND delete_status = ? ORDER BY update_time DESC"
	err := m.QueryRowsNoCache(&carts, query, userId, 0)
	if err != nil {
		return nil, err
	}
	return carts, nil
}
