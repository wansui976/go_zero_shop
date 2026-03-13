package model

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var _ OrdersModel = (*customOrdersModel)(nil)

type (
	// OrdersModel is an interface to be customized, add more methods here,
	// and implement the added methods in customOrdersModel.
	OrdersModel interface {
		ordersModel
		TxInsert(ctx context.Context, tx *sql.Tx, order *Orders) error
		TxGetById(ctx context.Context, tx *sql.Tx, orderId string) (*Orders, error)
		FindListByUserId(ctx context.Context, userId int64, status int64) ([]*Orders, error)
		// UpdateStatusIf performs optimistic update: set status=newStatus where id=id and status=oldStatus
		UpdateStatusIf(ctx context.Context, id, oldStatus, newStatus int64) (int64, error)
		// InsertStatusLog records a status change log
		InsertStatusLog(ctx context.Context, log *OrderStatusLog) error
	}

	customOrdersModel struct {
		*defaultOrdersModel
	}
)

// NewOrdersModel returns a model for the orderbase table.
func NewOrdersModel(conn sqlx.SqlConn, c cache.CacheConf, opts ...cache.Option) OrdersModel {
	return &customOrdersModel{
		defaultOrdersModel: newOrdersModel(conn, c, opts...),
	}
}
func (m *customOrdersModel) TxInsert(
	ctx context.Context,
	tx *sql.Tx,
	order *Orders,
) error {
	// 1. 插入订单主表（orders）
	// 注意：create_time/update_time 数据库自动填充，无需手动赋值
	query := fmt.Sprintf(
		"INSERT INTO %s (id, order_id, user_id, total_price, payment_type, status, receiver_name, receiver_phone, receiver_address, gid, create_time, update_time) "+
			"VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		m.table,
	)

	// 2. 修改参数列表，一一对应上面的问号
	_, err := tx.ExecContext(
		ctx,
		query,
		order.Id,              // 对应 id
		order.OrderId,         // 对应 order_id
		order.UserId,          // 对应 user_id
		order.TotalPrice,      // 对应 total_price
		order.PaymentType,     // 对应 payment_type
		order.Status,          // 对应 status
		order.ReceiverName,    // receiver_name
		order.ReceiverPhone,   // receiver_phone
		order.ReceiverAddress, // receiver_address
		order.Gid,             // 对应 gid (建议在 Logic 层赋值，方便排查)
		order.CreateTime,      // 对应 create_time
		order.CreateTime,      // 对应 update_time (插入时 update_time 通常等于 create_time)
	)

	if err != nil {
		return fmt.Errorf("插入订单主表失败: %w", err)
	}

	return nil
}

func (m *customOrdersModel) TxGetById(
	ctx context.Context,
	tx *sql.Tx,
	orderId string,
) (*Orders, error) {
	// 1. 构建查询SQL（只查询业务所需核心字段，避免冗余；字段顺序与结构体一致）
	query := `
	SELECT id, user_id, total_price, payment_type, status, order_id,receiver_name,receiver_phone,receiver_address
	FROM orders
	WHERE order_id = ?
	`

	// 2. 初始化订单模型（用于接收查询结果）
	var order Orders

	// 3. 执行事务内查询（tx.QueryRowContext，而非 conn.QueryRowContext）
	err := tx.QueryRowContext(ctx, query, orderId).Scan(
		&order.Id,          // 订单ID
		&order.UserId,      // 用户ID
		&order.TotalPrice,  // 订单总价
		&order.PaymentType, // 支付类型
		&order.Status,      // 订单状态
		&order.OrderId,     // 订单唯一编号
		&order.ReceiverName,
		&order.ReceiverPhone,
		&order.ReceiverAddress,
	)

	// 4. 错误处理（关键：区分「订单不存在」和「查询失败」）
	switch {
	case err == sql.ErrNoRows:
		// 订单不存在：返回 nil, nil（业务层可通过判断订单是否为nil处理）
		logx.Infof("TxGetById: 订单不存在(orderId:%s)", orderId)
		return nil, nil
	case err != nil:
		// 其他错误（如SQL语法错误、数据库连接异常等）：包装错误返回
		errMsg := fmt.Sprintf("TxGetById 查询失败(orderId:%s): %v", orderId, err)
		logx.Error(errMsg)
		return nil, fmt.Errorf("%s", errMsg)
	default:
		// 查询成功：返回订单模型
		logx.Debugf("TxGetById: 查询成功(orderId:%s, userId:%d, status:%d)", order.OrderId, order.UserId, order.Status)
		return &order, nil
	}
}

func (m *customOrdersModel) FindListByUserId(
	ctx context.Context,
	userId int64,
	status int64,
) ([]*Orders, error) {
	// 构建查询SQL，返回所有匹配的订单
	var query string
	var args []interface{}
	var whereClauses []string

	// 添加基础查询条件
	whereClauses = append(whereClauses, "user_id = ?")
	args = append(args, userId)

	// 添加状态过滤条件（如果status不是0）
	if status != 0 {
		whereClauses = append(whereClauses, "status = ?")
		args = append(args, status)
	}

	// 构建完整查询
	whereStr := strings.Join(whereClauses, " AND ")
	query = fmt.Sprintf("SELECT %s FROM %s WHERE %s ORDER BY id DESC", ordersRows, m.table, whereStr)

	// 执行查询
	var orders []*Orders
	err := m.QueryRowsNoCacheCtx(ctx, &orders, query, args...)
	if err != nil {
		return nil, fmt.Errorf("FindListByUserId 查询失败: %w", err)
	}

	return orders, nil
}

// UpdateStatusIf performs optimistic-lock update and returns affected rows
func (m *customOrdersModel) UpdateStatusIf(ctx context.Context, id, oldStatus, newStatus int64) (int64, error) {
	query := fmt.Sprintf("UPDATE %s SET status = ?, update_time = ? WHERE id = ? AND status = ?", m.table)
	res, err := m.ExecCtx(ctx, func(ctx context.Context, conn sqlx.SqlConn) (result sql.Result, err error) {
		return conn.ExecCtx(ctx, query, newStatus, time.Now().UnixMilli(), id, oldStatus)
	})
	if err != nil {
		return 0, err
	}
	rows, _ := res.RowsAffected()
	return rows, nil
}

// InsertStatusLog inserts an order status change record
func (m *customOrdersModel) InsertStatusLog(ctx context.Context, logEntry *OrderStatusLog) error {
	query := "INSERT INTO order_status_log (order_id, old_status, new_status, operator, reason, create_time) VALUES (?, ?, ?, ?, ?, ?)"
	_, err := m.ExecCtx(ctx, func(ctx context.Context, conn sqlx.SqlConn) (result sql.Result, err error) {
		return conn.ExecCtx(ctx, query, logEntry.OrderId, logEntry.OldStatus, logEntry.NewStatus, logEntry.Operator, logEntry.Reason, logEntry.CreateTime.UnixMilli())
	})
	return err
}
