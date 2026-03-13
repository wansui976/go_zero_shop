package logic

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/dtm-labs/dtmgrpc"
	"github.com/wansui976/go_zero_shop/apps/order/rpc/internal/svc"
	"github.com/wansui976/go_zero_shop/apps/order/rpc/order"
	"github.com/wansui976/go_zero_shop/pkg/xerr"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateOrderDTMConfirmLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateOrderDTMConfirmLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateOrderDTMConfirmLogic {
	return &CreateOrderDTMConfirmLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// Confirm：确认订单（扣库存 + 改状态）

func (l *CreateOrderDTMConfirmLogic) CreateOrderDTMConfirm(in *order.CreateOrderDTMConfirmRequest) (*order.CreateOrderDTMConfirmResponse, error) {
	// 1. 入参校验（DTM 关键参数不能为空）
	if in.OrderId == "" {
		logx.Errorf("CreateOrderDTMConfirm: 订单ID为空(gid:%s)", in.Gid)
		return nil, status.Error(codes.InvalidArgument, "订单ID不能为空")
	}
	if in.Gid == "" {
		logx.Errorf("CreateOrderDTMConfirm: DTM全局事务ID(gid)为空(orderId:%s)", in.OrderId)
		return nil, status.Error(codes.InvalidArgument, "全局事务ID不能为空")
	}

	// 2. DTM Barrier 初始化（核心：保证幂等性、防悬挂、防空补偿）
	// Barrier 会自动校验 gid+orderId 的唯一性，避免重复执行 Confirm 逻辑
	barrier, err := dtmgrpc.BarrierFromGrpc(l.ctx)
	if err != nil {
		logx.Errorf("CreateOrderDTMConfirm: Barrier初始化失败(gid:%s, orderId:%s):%v", in.Gid, in.OrderId, err)
		return nil, status.Error(codes.Internal, "分布式事务确认阶段初始化失败")
	}

	// 3. 绑定数据库事务执行 Confirm 逻辑（即使无写操作，也需通过 Barrier 确保事务一致性）
	db := l.svcCtx.DB
	err = barrier.CallWithDB(db, func(tx *sql.Tx) error {
		// 核心逻辑：校验 Try 阶段的订单是否已成功创建（确保 Try 执行成功）
		// 无需重复执行写操作（Try 已完成订单插入、库存扣减），仅做状态确认
		orderModel, err := l.svcCtx.OrderModel.TxGetById(l.ctx, tx, in.OrderId)
		if err != nil {
			// 订单查询失败（非不存在），返回错误触发 DTM 重试
			return fmt.Errorf("查询订单失败(orderId:%s):%w", in.OrderId, err)
		}
		if orderModel == nil {
			// 订单不存在，说明 Try 阶段执行失败，Confirm 阶段也应失败（触发 DTM 补偿）
			return fmt.Errorf("订单不存在(orderId:%s)，Try阶段未执行成功", in.OrderId)
		}
		if !orderModel.Gid.Valid || orderModel.Gid.String != in.Gid {
			return fmt.Errorf("订单GID不匹配(orderId:%s, expect:%s, actual:%s)", in.OrderId, in.Gid, orderModel.Gid.String)
		}

		// 可选：校验订单状态（确保 Try 阶段设置的状态正确，避免异常）
		if orderModel.Status != 1 { // 1=待支付（与 Try 阶段一致）
			return fmt.Errorf("订单状态异常(orderId:%s, status:%d)，无法确认", in.OrderId, orderModel.Status)
		}

		// 显式确认预扣库存生命周期（pre_locked -> confirmed）
		if l.svcCtx != nil && l.svcCtx.Rdb != nil {
			items, err := l.svcCtx.OrderitemModel.TxListByOrderId(l.ctx, tx, in.OrderId)
			if err != nil {
				return fmt.Errorf("查询订单项失败(orderId:%s): %w", in.OrderId, err)
			}
			for _, item := range items {
				if err := confirmPreLockStockByGID(l.ctx, l.svcCtx.Rdb, item.ProductId, in.Gid); err != nil {
					return fmt.Errorf("确认预扣库存失败(orderId:%s, productId:%d): %w", in.OrderId, item.ProductId, err)
				}
			}
		}

		// 4. 尝试从 Redis 中读取 gid -> requestId 映射，若存在则持久化 request_id -> order_id
		if l.svcCtx != nil && l.svcCtx.Rdb != nil {
			if reqId, err := l.svcCtx.Rdb.Get(l.ctx, "order:gid_to_request:"+in.Gid).Result(); err == nil && reqId != "" {
				// 插入映射表（幂等插入）
				_, err := tx.ExecContext(l.ctx,
					`INSERT INTO order_request_mapping (request_id, order_id, create_time)
					 VALUES (?, ?, ?)
					 ON DUPLICATE KEY UPDATE order_id = VALUES(order_id)`,
					reqId, in.OrderId, time.Now().UnixMilli(),
				)
				if err != nil {
					return fmt.Errorf("持久化 request_id->order_id 失败: %w", err)
				}
			}
		}

		logx.Infof("CreateOrderDTMConfirm: 分布式事务确认成功(gid:%s, orderId:%s)", in.Gid, in.OrderId)
		return nil
	})

	// 4. 错误处理
	if err != nil {
		logx.Errorf("CreateOrderDTMConfirm: 确认失败(gid:%s, orderId:%s):%v", in.Gid, in.OrderId, err)
		// 返回业务错误码，DTM 会感知并触发重试（默认重试3次）
		return nil, xerr.NewErrMsg(fmt.Sprintf("订单事务确认失败: %v", err))
	}

	// 5. 返回成功响应
	return &order.CreateOrderDTMConfirmResponse{
		Success: true,
	}, nil
}
