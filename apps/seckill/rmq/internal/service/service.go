package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/dtm-labs/dtmgrpc"
	"github.com/wansui976/go_zero_shop/apps/order/rpc/order"
	"github.com/wansui976/go_zero_shop/apps/product/rpc/product"
	"github.com/wansui976/go_zero_shop/apps/seckill/rmq/internal/config"
	"github.com/wansui976/go_zero_shop/pkg/traceutil"
	"github.com/zeromicro/go-zero/core/contextx"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/zrpc"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	otelcodes "go.opentelemetry.io/otel/codes"
	"google.golang.org/grpc/status"
)

const (
	chanCount   = 10               // 消息分片通道数量（控制并发度）
	bufferCount = 1024             // 每个通道缓冲区大小（应对流量峰值）
	rpcTimeout  = 3 * time.Second  // RPC调用超时时间（避免阻塞）
	tccTimeout  = 10 * time.Second // DTM TCC事务超时时间
)

type Service struct {
	c          config.Config            // 服务配置
	ProductRPC product.ProductClient    // 商品RPC客户端
	OrderRPC   order.OrderServiceClient // 订单RPC客户端
	waiter     sync.WaitGroup           // 等待组（优雅关闭）
	msgsChan   []chan *queuedKafkaData  // 消息分片通道数组
	ctx        context.Context          // 全局上下文（用于关闭信号）
	cancel     context.CancelFunc       // 取消函数（触发协程退出）
	dtmServer  string                   // DTM协调器地址（从配置读取）
}

// KafkaData Kafka消息结构化数据（秒杀请求：用户ID+商品ID）
type KafkaData struct {
	Uid   int64             `json:"uid"`             // 用户ID（必须>0）
	Pid   int64             `json:"pid"`             // 商品ID（必须>0）
	Trace map[string]string `json:"trace,omitempty"` // trace 上下文
}

type queuedKafkaData struct {
	ctx  context.Context
	data *KafkaData
}

// NewService 创建秒杀消息处理服务实例
func NewService(c config.Config) *Service {
	// 初始化全局上下文（用于优雅关闭）
	ctx, cancel := context.WithCancel(context.Background())

	// 从配置读取DTM协调器地址（避免硬编码）
	if c.DtmServer == "" {
		logx.Errorf("DTM server address is empty in config")
	}

	s := &Service{
		c:         c,
		ctx:       ctx,
		cancel:    cancel,
		dtmServer: c.DtmServer,
		msgsChan:  make([]chan *queuedKafkaData, chanCount),
	}

	// 初始化商品RPC客户端（带超时配置）
	s.ProductRPC = s.initProductRPC()
	// 初始化订单RPC客户端（带超时配置）
	s.OrderRPC = s.initOrderRPC()

	// 初始化消息通道并启动消费协程
	for i := 0; i < chanCount; i++ {
		ch := make(chan *queuedKafkaData, bufferCount)
		s.msgsChan[i] = ch

		s.waiter.Add(1)
		// 启动TCC模式消费协程（传递分片索引，便于日志排查）
		go s.consumeDTM(ch, i)
	}

	// 注册信号处理（监听关闭信号，触发优雅退出）
	s.registerSignalHandler()

	return s
}

// initProductRPC 初始化商品RPC客户端（带超时和连接配置）
func (s *Service) initProductRPC() product.ProductClient {
	client := zrpc.MustNewClient(s.c.ProductRPC,
		zrpc.WithTimeout(rpcTimeout), // RPC超时配置
		//zrpc.WithDialOption(transport.WithDialTimeout(5*time.Second)), // 连接超时
	)
	return product.NewProductClient(client.Conn())
}

// initOrderRPC 初始化订单RPC客户端（带超时和连接配置）
func (s *Service) initOrderRPC() order.OrderServiceClient {
	client := zrpc.MustNewClient(s.c.OrderRPC,
		zrpc.WithTimeout(rpcTimeout),
		//zrpc.WithDialOption(transport.WithDialTimeout(5*time.Second)),
	)
	return order.NewOrderServiceClient(client.Conn())
}

// registerSignalHandler 注册信号处理（优雅关闭）
func (s *Service) registerSignalHandler() {
	go func() {
		sigChan := make(chan os.Signal, 1)
		// 监听常见关闭信号：SIGINT（Ctrl+C）、SIGTERM（kill命令）
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

		select {
		case sig := <-sigChan:
			logx.Infof("received shutdown signal: %v, starting graceful shutdown", sig)
			// 触发协程退出
			s.cancel()
			// 关闭所有消息通道
			for _, ch := range s.msgsChan {
				close(ch)
			}
			// 等待所有消费协程退出（最多等待10秒）
			done := make(chan struct{})
			go func() {
				s.waiter.Wait()
				close(done)
			}()
			select {
			case <-done:
				logx.Infof("all consume goroutines exited successfully")
			case <-time.After(10 * time.Second):
				logx.Infof("graceful shutdown timeout, some goroutines may still running")
			}
			logx.Infof("service shutdown completed")
		case <-s.ctx.Done():
			return
		}
	}()
}

// consumeDTM DTM TCC分布式事务消费模式（核心逻辑）
// 参数：ch - 消息通道；chanIdx - 通道索引（日志排查用）
func (s *Service) consumeDTM(ch chan *queuedKafkaData, chanIdx int) {
	defer s.waiter.Done()
	logx.Infof("consumeDTM goroutine started (chanIdx:%d)", chanIdx)

	// 初始化RPC服务地址（从配置构建）
	productServer, err := s.c.ProductRPC.BuildTarget()
	if err != nil {
		logx.Errorf("chanIdx:%d build product server address error: %v", chanIdx, err)
		return
	}
	orderServer, err := s.c.OrderRPC.BuildTarget()
	if err != nil {
		logx.Errorf("chanIdx:%d build order server address error: %v", chanIdx, err)
		return
	}

	// 循环消费消息（直到上下文取消或通道关闭）
	for {
		select {
		case <-s.ctx.Done():
			logx.Infof("chanIdx:%d consumeDTM goroutine exiting (context canceled)", chanIdx)
			return
		case msg, ok := <-ch:
			if !ok {
				logx.Infof("chanIdx:%d consumeDTM goroutine exiting (channel closed)", chanIdx)
				return
			}
			m := msg.data

			// 消息数据校验（避免无效请求）
			if err := s.validateKafkaData(m, chanIdx); err != nil {
				logx.Errorf("chanIdx:%d invalid kafka data: %v, data:%+v", chanIdx, err, m)
				continue // 跳过无效消息，继续消费后续
			}
			logx.Infof("chanIdx:%d receive seckill request: uid:%d, pid:%d", chanIdx, m.Uid, m.Pid)

			// 执行DTM TCC全局事务
			if err := s.executeTccTransaction(msg.ctx, m, productServer, orderServer, chanIdx); err != nil {
				// 事务失败：记录错误日志（DTM会自动重试，无需终止协程）
				logx.Errorf("chanIdx:%d execute TCC transaction failed (uid:%d, pid:%d): %v", chanIdx, m.Uid, m.Pid, err)
				continue
			}
		}
	}
}

// validateKafkaData 校验Kafka消息数据合法性
func (s *Service) validateKafkaData(data *KafkaData, chanIdx int) error {
	if data == nil {
		return fmt.Errorf("data is nil")
	}
	if data.Uid <= 0 {
		return fmt.Errorf("invalid uid: %d (must >0)", data.Uid)
	}
	if data.Pid <= 0 {
		return fmt.Errorf("invalid pid: %d (must >0)", data.Pid)
	}
	return nil
}

// executeTccTransaction 执行DTM TCC全局事务
func (s *Service) executeTccTransaction(
	ctx context.Context,
	data *KafkaData,
	productServer, orderServer string,
	chanIdx int,
) error {
	dtmTracer := otel.Tracer("go-zero-shop/dtm")

	if ctx == nil {
		ctx = context.Background()
	}
	ctx = contextx.ValueOnlyFrom(ctx)

	// 生成DTM全局事务ID（GID）：全局唯一，关联秒杀请求
	_, gidSpan := dtmTracer.Start(ctx, "dtm.tcc.must_gen_gid")
	gidSpan.SetAttributes(
		attribute.String("dtm.server", s.dtmServer),
		attribute.Int("messaging.partition.channel", chanIdx),
		attribute.Int64("user.id", data.Uid),
		attribute.Int64("product.id", data.Pid),
	)
	gid := dtmgrpc.MustGenGid(s.dtmServer)
	gidSpan.SetAttributes(attribute.String("dtm.gid", gid))
	gidSpan.End()

	logx.Infof("chanIdx:%d start TCC transaction (gid:%s, uid:%d, pid:%d)", chanIdx, gid, data.Uid, data.Pid)

	// 构建带超时和追踪的上下文
	txCtx, cancel := context.WithTimeout(ctx, tccTimeout)
	defer cancel()

	// 发起DTM TCC全局事务
	txCtx, txSpan := dtmTracer.Start(txCtx, "dtm.tcc.transaction")
	txSpan.SetAttributes(
		attribute.String("dtm.gid", gid),
		attribute.String("dtm.server", s.dtmServer),
		attribute.Int("messaging.partition.channel", chanIdx),
		attribute.Int64("user.id", data.Uid),
		attribute.Int64("product.id", data.Pid),
	)
	err := dtmgrpc.TccGlobalTransaction(s.dtmServer, gid, func(tcc *dtmgrpc.TccGrpc) error {
		// -------------------------- TCC分支1：商品库存操作（Try-Confirm-Cancel） --------------------------
		// Try：检查库存是否充足 + 预扣库存（预留资源）
		// Confirm：确认预扣库存（正式扣减）
		// Cancel：回滚预扣库存（恢复资源）
		productTryReq := &product.UpdateProductStockRequest{
			ProductId: data.Pid,
			Num:       1, // 秒杀默认扣减1件（可从配置/消息中读取）
		}
		_, productBranchSpan := dtmTracer.Start(txCtx, "dtm.tcc.branch.product")
		productBranchSpan.SetAttributes(
			attribute.String("dtm.gid", gid),
			attribute.String("rpc.system", "grpc"),
			attribute.String("rpc.service", "product.Product"),
			attribute.Int64("product.id", data.Pid),
			attribute.Int64("user.id", data.Uid),
		)
		if err := tcc.CallBranch(
			productTryReq,
			productServer+"/product.Product/CheckAndReserveStock", // Try：检查+预扣
			productServer+"/product.Product/ConfirmStockDeduct",   // Confirm：确认扣减
			productServer+"/product.Product/CancelStockReserve",   // Cancel：回滚预扣
			&product.UpdateProductStockResponse{},
		); err != nil {
			productBranchSpan.RecordError(err)
			productBranchSpan.SetStatus(otelcodes.Error, err.Error())
			productBranchSpan.End()
			// 商品分支失败：返回错误，DTM会触发全局回滚
			return fmt.Errorf("product TCC branch failed: %w", err)
		}
		productBranchSpan.End()

		// -------------------------- TCC分支2：订单创建操作（Try-Confirm-Cancel） --------------------------
		// Try：检查用户是否重复秒杀 + 创建预订单（预留订单资源）
		// Confirm：确认创建正式订单
		// Cancel：删除预订单
		orderTryReq := &order.CreateOrderRequest{
			UserId:    data.Uid, // 秒杀用户ID（与 data.Uid 对应）
			AddressId: 0,        // 注意：需替换为实际收货地址ID（如从用户默认地址查询/ Kafka消息携带）
			UseDtm:    true,     // 必须设为true，告诉订单服务走 DTM 事务
			// Items：秒杀单商品，包装为数组（适配多商品结构）
			Items: []*order.OrderProductItem{
				{
					ProductId: data.Pid, // 秒杀商品ID（与 data.Pid 对应）
					Quantity:  1,        // 秒杀默认买1件

				},
			},
		}
		_, orderBranchSpan := dtmTracer.Start(txCtx, "dtm.tcc.branch.order")
		orderBranchSpan.SetAttributes(
			attribute.String("dtm.gid", gid),
			attribute.String("rpc.system", "grpc"),
			attribute.String("rpc.service", "order.Order"),
			attribute.Int64("product.id", data.Pid),
			attribute.Int64("user.id", data.Uid),
		)
		if err := tcc.CallBranch(
			orderTryReq,
			orderServer+"/order.Order/TryCreateOrder", // Try：检查+创建预订单
			orderServer+"/order.Order/ConfirmOrder",   // Confirm：创建正式订单
			orderServer+"/order.Order/CancelOrder",    // Cancel：删除预订单
			&order.CreateOrderResponse{},
		); err != nil {
			orderBranchSpan.RecordError(err)
			orderBranchSpan.SetStatus(otelcodes.Error, err.Error())
			orderBranchSpan.End()
			// 订单分支失败：返回错误，DTM会回滚所有已执行分支
			return fmt.Errorf("order TCC branch failed: %w", err)
		}
		orderBranchSpan.End()

		// 所有Try分支成功：返回nil，DTM自动执行Confirm
		return nil
	})

	if err != nil {
		txSpan.RecordError(err)
		txSpan.SetStatus(otelcodes.Error, err.Error())
		txSpan.End()
		// 解析DTM错误（区分业务错误和系统错误）
		if st, ok := status.FromError(err); ok {
			return fmt.Errorf("DTM TCC transaction failed (gid:%s, code:%s, message:%s)",
				gid, st.Code(), st.Message())
		}
		return fmt.Errorf("DTM TCC transaction failed (gid:%s): %w", gid, err)
	}
	txSpan.End()

	logx.Infof("chanIdx:%d TCC transaction success (gid:%s, uid:%d, pid:%d)", chanIdx, gid, data.Uid, data.Pid)
	return nil
}

// Consume Kafka消息消费入口（实现消费者接口）
func (s *Service) Consume(ctx context.Context, _ string, value string) error {
	logx.Debugf("received kafka raw message: %s", value)

	// 解析Kafka消息（兼容单条消息和批量消息）
	var dataList []*KafkaData
	// 尝试解析为数组
	if err := json.Unmarshal([]byte(value), &dataList); err != nil {
		// 解析数组失败，尝试解析为单条消息
		var singleData KafkaData
		if err := json.Unmarshal([]byte(value), &singleData); err != nil {
			return fmt.Errorf("json unmarshal failed: %w, raw value:%s", err, value)
		}
		dataList = []*KafkaData{&singleData}
	}

	// 分发消息到分片通道（按Pid取模，同一商品串行处理）
	for _, data := range dataList {
		msgCtx := traceutil.ExtractContext(ctx, data.Trace)
		msgCtx = contextx.ValueOnlyFrom(msgCtx)
		// 非阻塞发送（避免通道满导致阻塞Consume）
		select {
		case s.msgsChan[data.Pid%chanCount] <- &queuedKafkaData{ctx: msgCtx, data: data}:
			logx.Debugf("dispatch seckill request: uid:%d, pid:%d, chanIdx:%d",
				data.Uid, data.Pid, data.Pid%chanCount)
		default:
			// 通道缓冲区满：记录告警日志（可根据业务配置重试或丢弃）
			logx.Infof("kafka message channel is full, discard request: uid:%d, pid:%d", data.Uid, data.Pid)
		}
	}

	return nil
}

// Close 服务关闭方法（供外部调用）
func (s *Service) Close() error {
	logx.Infof("service close triggered")
	s.cancel()
	for _, ch := range s.msgsChan {
		close(ch)
	}
	s.waiter.Wait()
	logx.Infof("service closed successfully")
	return nil
}
