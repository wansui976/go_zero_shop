package logic

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/dtm-labs/dtmgrpc"
	"github.com/wansui976/go_zero_shop/apps/order/rpc/internal/svc"
	"github.com/wansui976/go_zero_shop/apps/order/rpc/order"
	//"github.com/wansui976/go_zero_shop/pkg/xerr"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateOrderDTMRevertLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateOrderDTMRevertLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateOrderDTMRevertLogic {
	return &CreateOrderDTMRevertLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// Cancel：回滚订单

func (l *CreateOrderDTMRevertLogic) CreateOrderDTMRevert(in *order.AddOrderRequest) (*order.AddOrderResponse, error) {
	if in.Gid == "" {
		logx.Errorf("CreateOrderDTMRevert: Gid 为空 (userId:%d)", in.UserId)
		return nil, status.Error(codes.InvalidArgument, "全局事务ID不能为空")
	}
	if in.UserId <= 0 {
		logx.Errorf("CreateOrderDTMRevert: UserId 无效 (gid:%s)", in.Gid)
		return nil, status.Error(codes.InvalidArgument, "用户ID无效")
	}

	// Barrier 初始化
	//dtmgrpc.NewSagaGrpc()
	barrier, err := dtmgrpc.BarrierFromGrpc(l.ctx)
	if err != nil {
		logx.Errorf("CreateOrderDTMRevert: Barrier 初始化失败 (gid:%s): %v", in.Gid, err)
		//return nil, status.Error(codes.Internal, "分布式事务回滚阶段初始化失败")
		//return nil, status.Error(codes.Internal, err.Error())
		return nil, err
	}

	db := l.svcCtx.DB
	var orderIDText string
	err = barrier.CallWithDB(db, func(tx *sql.Tx) error {
		// 1) 通过 gid 查订单；若 Try 未落库则按请求参数释放 Redis 预扣库存。
		var orderPk int64
		var dbOrderID string
		var orderStatus int32
		var orderUserId int64
		row := tx.QueryRowContext(l.ctx, "SELECT id, order_id, user_id, status FROM orders WHERE gid = ? LIMIT 1", in.Gid)
		if err := row.Scan(&orderPk, &dbOrderID, &orderUserId, &orderStatus); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				// Try 未创建订单，仍需按请求释放预扣库存（幂等）
				if l.svcCtx != nil && l.svcCtx.Rdb != nil {
					for _, item := range in.Items {
						_ = revertPreLockStockByGID(l.ctx, l.svcCtx.Rdb, item.ProductId, in.Gid)
					}
				}
				logx.Infof("CreateOrderDTMRevert: 未找到订单(gid:%s)，按请求释放预扣库存完成", in.Gid)
				return nil
			}
			return fmt.Errorf("查询订单失败 (gid=%s): %w", in.Gid, err)
		}
		orderIDText = dbOrderID

		// 校验属主（兼容补偿调用未携带 user_id 的场景）
		if in.UserId > 0 && orderUserId != in.UserId {
			return fmt.Errorf("订单属主不匹配 (gid=%s, orderId=%d, actualUser=%d, reqUser=%d)",
				in.Gid, orderPk, orderUserId, in.UserId)
		}

		// 若订单已被取消（状态0视为已取消），则幂等返回
		if orderStatus == 0 {
			logx.Infof("CreateOrderDTMRevert: 订单已取消，无需重复更新(orderId=%d, gid=%s)", orderPk, in.Gid)
		} else {
			// 2) 更新订单状态为已取消（0）
			updRes, err := tx.ExecContext(
				l.ctx,
				"UPDATE orders SET status = 0, update_time = ? WHERE id = ? AND status <> 0",
				time.Now().UnixMilli(),
				orderPk,
			)
			if err != nil {
				return fmt.Errorf("更新订单状态失败(orderId=%d): %w", orderPk, err)
			}
			if aff, _ := updRes.RowsAffected(); aff == 0 {
				logx.Infof("CreateOrderDTMRevert: 订单状态已由并发请求处理(orderId=%d, gid=%s)", orderPk, in.Gid)
			}
		}

		// 3) 优先回滚请求携带的商品预扣；若请求未携带，则从订单项中查询补偿。
		if l.svcCtx != nil && l.svcCtx.Rdb != nil {
			items := in.Items
			if len(items) == 0 {
				orderItems, err := l.svcCtx.OrderitemModel.TxListByOrderId(l.ctx, tx, dbOrderID)
				if err != nil {
					return fmt.Errorf("查询订单项失败(orderId=%s): %w", dbOrderID, err)
				}
				items = make([]*order.OrderProductItem, 0, len(orderItems))
				for _, it := range orderItems {
					items = append(items, &order.OrderProductItem{
						ProductId: it.ProductId,
						Quantity:  it.Quantity,
					})
				}
			}
			for _, item := range items {
				if err := revertPreLockStockByGID(l.ctx, l.svcCtx.Rdb, item.ProductId, in.Gid); err != nil {
					return fmt.Errorf("回滚预扣库存失败(productId=%d): %w", item.ProductId, err)
				}
			}
		}

		logx.Infof("CreateOrderDTMRevert: 回滚完成(gid=%s, orderId=%s, userId=%d)", in.Gid, dbOrderID, in.UserId)
		return nil
	})

	if err != nil {
		logx.Errorf("CreateOrderDTMRevert: 回滚失败(gid=%s): %v", in.Gid, err)
		//return nil, xerr.NewErrMsg(fmt.Sprintf("订单事务回滚失败: %v", err))
		//return nil, status.Error(codes.Internal, err.Error())
		return nil, err

	}

	return &order.AddOrderResponse{
		OrderId: orderIDText,
	}, nil
}
