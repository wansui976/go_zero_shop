package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/wansui976/go_zero_shop/pkg/mq"
	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

// 订单创建事件
type OrderCreatedEvent struct {
	Event   string      `json:"event"`
	OrderID int64       `json:"order_id"`
	UserID  int64       `json:"user_id"`
	Time    time.Time   `json:"time"`
	Items   []OrderItem `json:"items"`
}

type OrderItem struct {
	ProductID int64 `json:"product_id"`
	Quantity  int64 `json:"quantity"`
}

type OrderConfig struct {
	service.ServiceConf
	RabbitMQ struct {
		URL      string `json:"Url"`
		PoolSize int    `json:"PoolSize"`
	}
	Redis struct {
		Host string `json:"Host"`
		Type string `json:"Type"`
		Pass string `json:"Pass"`
	}
}

const (
	ORDER_QUEUE = "order_create_queue"
)

// ==================== Prometheus 监控指标 ====================
var (
	// 消费消息数
	orderConsumeCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "order_consumer_messages_total",
		Help: "Total number of order created messages consumed",
	}, []string{"action", "status"})

	// 消费耗时
	orderConsumeDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "order_consumer_duration_seconds",
		Help:    "Duration of order message processing",
		Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1, 5},
	})

	// 队列堆积数
	orderQueueGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "order_consumer_queue_depth",
		Help: "Current depth of order create queue",
	})
)

func main() {
	var configFile = flag.String("f", "etc/order-consumer.yaml", "配置文件路径")
	flag.Parse()

	var c OrderConfig
	conf.MustLoad(*configFile, &c)

	ctx := context.Background()

	// 初始化 RabbitMQ
	rabbitMQ, err := mq.NewRabbitMQ(mq.RabbitMQConfig{
		URL:      c.RabbitMQ.URL,
		PoolSize: c.RabbitMQ.PoolSize,
	})
	if err != nil {
		logx.Errorf("初始化RabbitMQ失败: %v", err)
		return
	}
	defer rabbitMQ.Close()

	// 初始化 Redis 客户端
	redisClient := redis.MustNewRedis(redis.RedisConf{
		Host: c.Redis.Host,
		Type: c.Redis.Type,
		Pass: c.Redis.Pass,
	})

	// 声明队列
	if err := rabbitMQ.DeclareQueue(ORDER_QUEUE, true, false, false, false, nil); err != nil {
		logx.Errorf("声明队列失败: %v", err)
		return
	}

	// 设置 QoS - 每次只预取 10 条消息
	if err := rabbitMQ.Qos(10, 0, false); err != nil {
		logx.Errorf("设置QoS失败: %v", err)
		return
	}

	// 启动队列深度监控
	go monitorOrderQueueDepth(rabbitMQ)

	// 开始消费
	deliveryChan, err := rabbitMQ.Consume(
		ORDER_QUEUE,
		"order-consumer",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		logx.Errorf("启动消费者失败: %v", err)
		return
	}

	logx.Info("订单消费者启动成功，等待消息...")

	// 处理消息
	for msg := range deliveryChan {
		start := time.Now()
		if err := handleOrderCreatedEvent(ctx, redisClient, msg); err != nil {
			logx.Errorf("处理消息失败: %v", err)
			orderConsumeCounter.WithLabelValues("process", "failed").Inc()
			msg.Nack(false, true) // 重新入队
		} else {
			msg.Ack(false)
			orderConsumeCounter.WithLabelValues("process", "success").Inc()
		}
		orderConsumeDuration.Observe(time.Since(start).Seconds())
	}
}

// monitorOrderQueueDepth 监控订单队列深度
func monitorOrderQueueDepth(r *mq.RabbitMQ) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		ch, err := r.GetChannel()
		if err != nil {
			continue
		}
		defer r.ReturnChannel(ch)

		queue, err := ch.QueueInspect(ORDER_QUEUE)
		if err != nil {
			continue
		}
		orderQueueGauge.Set(float64(queue.Messages))
	}
}

// handleOrderCreatedEvent 处理订单创建事件（带幂等性检查）
func handleOrderCreatedEvent(ctx context.Context, rdb *redis.Redis, msg amqp.Delivery) error {
	var event OrderCreatedEvent
	if err := json.Unmarshal(msg.Body, &event); err != nil {
		orderConsumeCounter.WithLabelValues("parse", "failed").Inc()
		return fmt.Errorf("解析消息失败: %w", err)
	}

	logx.Infof("收到订单创建事件: OrderID=%d, UserID=%d, Time=%v",
		event.OrderID, event.UserID, event.Time)

	// 幂等性检查 - 使用 OrderID 作为 key
	key := fmt.Sprintf("order_event:%d", event.OrderID)
	exists, _ := rdb.Exists(key)
	if exists {
		logx.Infof("重复的订单事件，跳过: OrderID=%d", event.OrderID)
		orderConsumeCounter.WithLabelValues("idempotent", "skipped").Inc()
		return nil
	}
	// 设置已处理标记（24小时过期）
	rdb.Setex(key, "1", 24*3600)

	// 缓存订单商品快照，供延迟取消链路回补库存使用。
	if err := cacheOrderItemsSnapshot(rdb, event.OrderID, event.Items); err != nil {
		logx.Errorf("缓存订单商品快照失败: OrderID=%d, err=%v", event.OrderID, err)
	}

	// 业务处理逻辑
	start := time.Now()
	defer func() {
		logx.Infof("订单事件处理完成: OrderID=%d, 耗时=%v", event.OrderID, time.Since(start))
	}()

	// 1. 发送订单确认通知
	if err := sendOrderConfirmationNotification(ctx, &event); err != nil {
		logx.Errorf("发送通知失败: %v", err)
		orderConsumeCounter.WithLabelValues("notification", "failed").Inc()
	} else {
		orderConsumeCounter.WithLabelValues("notification", "success").Inc()
	}

	// 2. 更新数据统计
	if err := updateOrderStatistics(ctx, &event); err != nil {
		logx.Errorf("更新统计失败: %v", err)
		orderConsumeCounter.WithLabelValues("statistics", "failed").Inc()
	} else {
		orderConsumeCounter.WithLabelValues("statistics", "success").Inc()
	}

	// 3. 触发库存预留
	if err := reserveInventory(ctx, &event); err != nil {
		logx.Errorf("预留库存失败: %v", err)
		orderConsumeCounter.WithLabelValues("inventory", "failed").Inc()
	} else {
		orderConsumeCounter.WithLabelValues("inventory", "success").Inc()
	}

	return nil
}

// sendOrderConfirmationNotification 发送订单确认通知
func sendOrderConfirmationNotification(ctx context.Context, event *OrderCreatedEvent) error {
	logx.Infof("发送订单确认通知给用户: UserID=%d, OrderID=%d", event.UserID, event.OrderID)

	// TODO: 实际项目中调用通知服务
	// 1. 发送短信通知
	// smsClient.Send(ctx, &sms.Request{Phone: user.Phone, Template: "order_confirm", Data: ...})

	// 2. 发送邮件通知
	// emailClient.Send(ctx, &email.Request{Email: user.Email, Subject: "订单确认", Body: ...})

	// 3. 记录通知日志
	logx.Infof("通知已发送: UserID=%d, OrderID=%d", event.UserID, event.OrderID)
	return nil
}

// updateOrderStatistics 更新订单统计数据
func updateOrderStatistics(ctx context.Context, event *OrderCreatedEvent) error {
	logx.Infof("更新订单统计数据: OrderID=%d, Items=%d", event.OrderID, len(event.Items))

	// TODO: 实际项目中更新 Redis 统计
	// 1. 日订单数 +1
	// incr date: order:stats:daily:2024-01-01

	// 2. 总订单数 +1
	// incr order:stats:total

	// 3. 热销商品统计
	// for _, item := range event.Items {
	//     zincrby order:stats:hot_products item.ProductID item.Quantity
	// }

	// 4. 销售额统计
	// TODO: 计算订单金额并累加

	logx.Infof("统计更新完成: OrderID=%d", event.OrderID)
	return nil
}

// reserveInventory 预留商品库存
func reserveInventory(ctx context.Context, event *OrderCreatedEvent) error {
	logx.Infof("预留商品库存: OrderID=%d, Items=%d", event.OrderID, len(event.Items))

	// TODO: 实际项目中调用库存服务预留库存
	// 1. 检查库存是否充足
	// 2. 预留库存（Redis 预扣）
	// 3. 记录预留日志

	for _, item := range event.Items {
		logx.Infof("预留库存: ProductID=%d, Qty=%d", item.ProductID, item.Quantity)
		// stockClient.Reserve(ctx, &stock.ReserveRequest{...})
	}

	return nil
}

func cacheOrderItemsSnapshot(rdb *redis.Redis, orderID int64, items []OrderItem) error {
	if rdb == nil || len(items) == 0 {
		return nil
	}
	data, err := json.Marshal(items)
	if err != nil {
		return err
	}
	key := fmt.Sprintf("order:items_snapshot:%d", orderID)
	// 超过延迟队列 TTL（30 分钟）即可，取 48 小时便于补偿排查。
	return rdb.Setex(key, string(data), 48*3600)
}
