package logic

import (
	"context"
	"fmt"
	"time"

	"github.com/wansui976/go_zero_shop/apps/order/rpc/internal/svc"
	"github.com/wansui976/go_zero_shop/apps/order/rpc/model"
	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateStatusLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateStatusLogic {
	return &UpdateStatusLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// UpdateOrderStatus 更新订单状态（含转换校验与乐观锁）
func (l *UpdateStatusLogic) UpdateOrderStatus(orderId int64, newStatus int64, operator, reason string) error {
	// 1. 查询当前订单
	orderModel, err := l.svcCtx.OrderModel.FindOne(l.ctx, orderId)
	if err != nil {
		return err
	}
	if orderModel == nil {
		return fmt.Errorf("order %d not found", orderId)
	}

	// 2. 校验状态转换
	if !orderModel.CanTransitionTo(newStatus) {
		return fmt.Errorf("cannot transition from %d to %d", orderModel.Status, newStatus)
	}

	// 3. 乐观更新
	affected, err := l.svcCtx.OrderModel.UpdateStatusIf(l.ctx, orderId, orderModel.Status, newStatus)
	if err != nil {
		return err
	}
	if affected == 0 {
		return fmt.Errorf("status changed concurrently, please retry")
	}

	// 4. 记录状态变更日志
	logEntry := &model.OrderStatusLog{
		OrderId:    orderId,
		OldStatus:  orderModel.Status,
		NewStatus:  newStatus,
		Operator:   operator,
		Reason:     reason,
		CreateTime: time.Now(),
	}
	if err := l.svcCtx.OrderModel.InsertStatusLog(l.ctx, logEntry); err != nil {
		// 日志写入失败不影响主流程，但记录错误
		l.Errorf("insert status log failed: %v", err)
	}

	return nil
}
