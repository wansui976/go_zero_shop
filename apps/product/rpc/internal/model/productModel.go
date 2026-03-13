package model

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ ProductModel = (*customProductModel)(nil)

type (
	ProductModel interface {
		productModel
		TxUpdateStock(tx *sql.Tx, productId int64, changeNum int64) (sql.Result, error)
		TxGetStock(tx *sql.Tx, productId int64) (int64, error)
		FindByCategory(ctx context.Context, ctime string, cateid, limit int64) ([]*Product, error)
		GetAll(ctx context.Context) ([]*Product, error)
		// FindTopBySort returns top products ordered by sort desc limited by limit
		FindTopBySort(ctx context.Context, limit int64) ([]*Product, error)
	}

	customProductModel struct {
		*defaultProductModel
	}
)

func NewProductModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) ProductModel {
	return &customProductModel{
		defaultProductModel: newProductModel(conn, c, opts...),
	}
}

// 事务性库存更新
func (m *customProductModel) TxUpdateStock(tx *sql.Tx, productId int64, changeNum int64) (sql.Result, error) {
	if productId <= 0 {
		return nil, errors.New("无效的商品ID")
	}
	if changeNum == 0 {
		return nil, errors.New("库存变化量不能为0")
	}

	var (
		sqlStr string
		args   []interface{}
	)

	if changeNum < 0 {
		sqlStr = `
			UPDATE product
			SET stock = stock + ?, update_time = NOW()
			WHERE id = ? AND status = 1 AND stock + ? >= 0
		`
		args = []interface{}{changeNum, productId, changeNum}
	} else {
		sqlStr = `
			UPDATE product
			SET stock = stock + ?, update_time = NOW()
			WHERE id = ? AND status = 1
		`
		args = []interface{}{changeNum, productId}
	}

	result, err := tx.Exec(sqlStr, args...)
	if err != nil {
		return nil, fmt.Errorf("执行库存更新失败: %w", err)
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		if changeNum < 0 {
			return nil, fmt.Errorf("商品 %d 库存不足", productId)
		}
		return nil, fmt.Errorf("商品 %d 不存在或已下架", productId)
	}

	logx.Infof("[TxUpdateStock] 成功: productId=%d, changeNum=%d", productId, changeNum)
	return result, nil
}

// 查询库存（事务内）
func (m *customProductModel) TxGetStock(tx *sql.Tx, productId int64) (int64, error) {
	if productId <= 0 {
		return 0, errors.New("无效的商品ID")
	}

	var stock int64
	err := tx.QueryRow("SELECT stock FROM product WHERE id = ? AND status = 1 LIMIT 1", productId).Scan(&stock)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, fmt.Errorf("商品 %d 不存在或已下架", productId)
		}
		return 0, fmt.Errorf("查询库存失败: %w", err)
	}
	return stock, nil
}

// 按分类查询商品
func (m *customProductModel) FindByCategory(ctx context.Context, ctime string, cateid, limit int64) ([]*Product, error) {
	var product []*Product
	err := m.QueryRowsNoCacheCtx(ctx, &product,
		fmt.Sprintf("SELECT %s FROM %s WHERE category_id=? AND status=1 AND create_time<? ORDER BY create_time DESC LIMIT ?", productRows, m.table),
		cateid, ctime, limit)
	if err != nil {
		return nil, err
	}
	return product, nil
}
func (m *customProductModel) GetAll(ctx context.Context) ([]*Product, error) {
	var product []*Product
	err := m.QueryRowsNoCacheCtx(ctx, &product,
		fmt.Sprintf("SELECT %s FROM %s WHERE status=1  ORDER BY create_time DESC", productRows, m.table))
	if err != nil {
		return nil, err
	}
	return product, nil
}

// FindTopBySort returns top products ordered by sort desc limited by limit
func (m *customProductModel) FindTopBySort(ctx context.Context, limit int64) ([]*Product, error) {
	var products []*Product
	err := m.QueryRowsNoCacheCtx(ctx, &products,
		fmt.Sprintf("SELECT %s FROM %s WHERE status=1 ORDER BY sort DESC LIMIT ?", productRows, m.table),
		limit)
	if err != nil {
		return nil, err
	}
	return products, nil
}
