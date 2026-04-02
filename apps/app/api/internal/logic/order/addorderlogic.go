package order

import (
	"context"
	"fmt"
	"strconv"

	"time"

	"github.com/dtm-labs/dtmgrpc"
	"github.com/wansui976/go_zero_shop/apps/app/api/internal/middleware"
	"github.com/wansui976/go_zero_shop/apps/app/api/internal/svc"
	"github.com/wansui976/go_zero_shop/apps/app/api/internal/types"
	"github.com/wansui976/go_zero_shop/apps/order/rpc/order"
	"github.com/wansui976/go_zero_shop/apps/product/rpc/product"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	// 订单项数量限制
	MaxOrderItems      = 50   // 单次订单最多商品种类
	MaxItemQuantity    = 9999 // 单个商品最大数量
	DefaultSagaTimeout = 60   // Saga事务超时时间（秒）
)

type AddOrderLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 创建订单
func NewAddOrderLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AddOrderLogic {
	return &AddOrderLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AddOrderLogic) AddOrder(req *types.OrderAddReq) (resp *types.CommonResponse, err error) {
	// 1. 提取并验证用户ID
	uid, err := l.extractUIDFromCtx()
	if err != nil {
		l.Errorf("提取用户ID失败: %v", err)
		return nil, status.Error(codes.Unauthenticated, "用户未登录或登录态失效")
	}

	// 幂等性检查（若前端提供 request_id）
	if req.RequestId != "" && l.svcCtx != nil && l.svcCtx.Idemp != nil {
		ok, err := l.svcCtx.Idemp.CheckAndSet(l.ctx, req.RequestId, 24*time.Hour)
		if err != nil {
			l.Errorf("幂等性检查失败: %v", err)
			return nil, status.Error(codes.Internal, "幂等性检查失败")
		}
		if !ok {
			// 已存在占位，尝试读取映射
			if l.svcCtx.Rdb != nil {
				if val, err := l.svcCtx.Rdb.Get(l.ctx, "order:mapping:"+req.RequestId).Result(); err == nil && val != "" {
					return &types.CommonResponse{
						ResultCode: 200,
						Msg:        "重复请求",
						Data:       map[string]interface{}{"gid": val},
					}, nil
				}
			}
			return &types.CommonResponse{ResultCode: 200, Msg: "重复请求，正在处理或已处理"}, nil
		}
	}

	// 2. 验证请求参数
	if err := l.validateOrderRequest(req); err != nil {
		l.Errorf("订单请求验证失败 uid=%d, err: %v", uid, err)
		return nil, err
	}

	// 3. 解析并验证地址ID
	addressId, err := strconv.ParseInt(req.ReceiveAddressId, 10, 64)
	if err != nil || addressId <= 0 {
		l.Errorf("收货地址ID无效 uid=%d, addressId=%s", uid, req.ReceiveAddressId)
		return nil, status.Error(codes.InvalidArgument, "收货地址ID无效")
	}

	// 4. 生成全局事务ID
	gid := dtmgrpc.MustGenGid(l.svcCtx.Config.DtmServer)
	l.Infof("开始创建订单 uid=%d, gid=%s, 商品种类=%d, 地址ID=%d",
		uid, gid, len(req.Items), addressId)

	// 5. 准备数据（单次循环，提高效率）
	productItems, orderItems := l.prepareOrderData(req.Items)

	// 6. 构建Saga事务
	saga := l.buildSagaTransaction(gid, uid, addressId, req.PaymentType, productItems, orderItems)

	// 7. 提交事务
	l.Infof("准备提交DTM事务 uid=%d, gid=%s", uid, gid)
	if err := saga.Submit(); err != nil {
		l.Errorf("DTM事务提交失败 uid=%d, gid=%s, err: %v", uid, gid, err)
		// 事务失败，释放幂等占位，允许重试
		if req.RequestId != "" && l.svcCtx != nil && l.svcCtx.Idemp != nil {
			_ = l.svcCtx.Idemp.Delete(l.ctx, req.RequestId)
		}
		return nil, status.Error(codes.Internal, "订单创建失败，请稍后重试")
	}

	l.Infof("订单创建成功 uid=%d, gid=%s", uid, gid)

	// 记录 request_id -> gid 映射，便于客户端查询幂等结果
	if req.RequestId != "" && l.svcCtx != nil && l.svcCtx.Rdb != nil {
		_ = l.svcCtx.Rdb.Set(l.ctx, "order:mapping:"+req.RequestId, gid, 24*time.Hour).Err()
		// 反向映射 gid -> requestId，供 order 服务在 Confirm 阶段持久化映射时查找
		_ = l.svcCtx.Rdb.Set(l.ctx, "order:gid_to_request:"+gid, req.RequestId, 24*time.Hour).Err()
	}

	// 8. 返回成功响应
	return &types.CommonResponse{
		ResultCode: 200,
		Msg:        "订单创建成功",
		Data: map[string]interface{}{
			"gid":     gid,
			"message": "订单处理中，请稍后查看订单状态",
		},
	}, nil
}

// 验证订单请求参数
func (l *AddOrderLogic) validateOrderRequest(req *types.OrderAddReq) error {
	// 验证订单项不能为空
	// 幂等逻辑请在 AddOrder 的上层实现（例如通过 request_id + Redis SETNX），
	// 此处只负责参数校验
	if len(req.Items) == 0 {
		return status.Error(codes.InvalidArgument, "订单商品不能为空")
	}

	// 验证订单项数量限制
	if len(req.Items) > MaxOrderItems {
		return status.Error(codes.InvalidArgument,
			fmt.Sprintf("单次订单商品种类不能超过%d", MaxOrderItems))
	}

	// 验证每个订单项
	productIdMap := make(map[int64]bool, len(req.Items))
	for i, item := range req.Items {
		// 验证商品ID
		if item.Id <= 0 {
			return status.Error(codes.InvalidArgument,
				fmt.Sprintf("第%d个商品ID无效", i+1))
		}

		// 检查是否有重复的商品ID
		if productIdMap[item.Id] {
			return status.Error(codes.InvalidArgument,
				fmt.Sprintf("商品ID %d 重复", item.Id))
		}
		productIdMap[item.Id] = true

		// 验证商品数量
		if item.Count <= 0 {
			return status.Error(codes.InvalidArgument,
				fmt.Sprintf("商品ID %d 的数量必须大于0", item.Id))
		}
		if item.Count > MaxItemQuantity {
			return status.Error(codes.InvalidArgument,
				fmt.Sprintf("商品ID %d 的数量不能超过%d", item.Id, MaxItemQuantity))
		}
	}

	// 验证支付类型
	if req.PaymentType != 1 && req.PaymentType != 2 {
		return status.Error(codes.InvalidArgument, "支付类型无效，仅支持微信支付(1)或支付宝支付(2)")
	}

	// 验证收货地址ID格式
	if req.ReceiveAddressId == "" {
		return status.Error(codes.InvalidArgument, "收货地址ID不能为空")
	}

	return nil
}

// 准备订单数据（合并循环，提高效率）
func (l *AddOrderLogic) prepareOrderData(items []*types.OrderItem) (
	[]*product.DecrProduct, []*order.OrderProductItem) {

	productItems := make([]*product.DecrProduct, 0, len(items))
	orderItems := make([]*order.OrderProductItem, 0, len(items))

	for _, item := range items {
		productItems = append(productItems, &product.DecrProduct{
			Id:  item.Id,
			Num: item.Count,
		})
		orderItems = append(orderItems, &order.OrderProductItem{
			ProductId: item.Id,
			Quantity:  item.Count,
		})
	}

	return productItems, orderItems
}

// 构建Saga事务
func (l *AddOrderLogic) buildSagaTransaction(
	gid string,
	uid int64,
	addressId int64,
	paymentType int8,
	productItems []*product.DecrProduct,
	orderItems []*order.OrderProductItem,
) *dtmgrpc.SagaGrpc {

	// 构建订单请求
	addOrderReq := &order.AddOrderRequest{
		UserId:      uid,
		AddressId:   addressId,
		Gid:         gid,
		PaymentType: int64(paymentType),
		Items:       orderItems,
	}

	// 构建库存扣减请求
	decrReq := &product.DecrStockRequest{
		Items: productItems,
	}

	orderService := l.svcCtx.Config.OrderServiceAddr
	productService := l.svcCtx.Config.ProductServiceAddr

	// 创建Saga事务
	// 注意：调整顺序，先扣减库存（更容易失败），再创建订单
	saga := dtmgrpc.NewSagaGrpc(l.svcCtx.Config.DtmServer, gid).
		Add(
			productService+"/product.Product/DecrStock",
			productService+"/product.Product/DecrStockRevert",
			decrReq,
		).
		Add(
			orderService+"/order.OrderService/CreateOrderDTM",
			orderService+"/order.OrderService/CreateOrderDTMRevert",
			addOrderReq,
		)

	return saga
}

// 从上下文中提取用户ID
func (l *AddOrderLogic) extractUIDFromCtx() (int64, error) {
	uid, err := middleware.GetUserIDFromCtx(l.ctx)
	if err != nil {
		return 0, err
	}

	if uid <= 0 {
		return 0, status.Error(codes.Unauthenticated, "用户ID无效")
	}

	return uid, nil
}
