package model

import "time"

// OrderStatusLog 订单状态变更日志模型
type OrderStatusLog struct {
	Id         int64     `db:"id"`
	OrderId    int64     `db:"order_id"`
	OldStatus  int64     `db:"old_status"`
	NewStatus  int64     `db:"new_status"`
	Operator   string    `db:"operator"`
	Reason     string    `db:"reason"`
	CreateTime time.Time `db:"create_time"`
}
