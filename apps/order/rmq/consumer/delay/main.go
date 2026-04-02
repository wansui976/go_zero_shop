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

// 订单延迟消息
type OrderDelayMessage struct {
	OrderID   int64       `json:"order_id"`
	UserID    int64       `json:"user_id"`
	Action    string      `json:"action"` // cancel, remind
	CreatedAt time.Time   `json:"created_at"`
	Items     []OrderItem `json:"items,omitempty"` // 可选：订单商品快照，用于取消时回补库存
}

type OrderItem struct {
	ProductID int64 `json:"product_id"`
	Quantity  int64 `json:"quantity"`
}

// 库存变更事件（与 stock consumer 保持一致）
type StockChangeEvent struct {
	EventID   string    `json:"event_id"`
	ProductID int64     `json:"product_id"`
	ChangeQty int64     `json:"change_qty"`
	OrderID   int64     `json:"order_id"`
	Reason    string    `json:"reason"` // order_create, order_cancel, restock
	Timestamp time.Time `json:"timestamp"`
}

// 订单通知事件（异步投递到通知队列）
type OrderNotificationEvent struct {
	EventID    string    `json:"event_id"`
	Type       string    `json:"type"` // order_cancelled, payment_reminder
	OrderID    int64     `json:"order_id"`
	UserID     int64     `json:"user_id"`
	ItemCount  int       `json:"item_count"`
	HasStockOp bool      `json:"has_stock_op"`
	CreatedAt  time.Time `json:"created_at"`
}

type DelayConfig struct {
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
	// 死信交换机
	DLX_EXCHANGE = "dlx.exchange"
	// 延迟队列（带TTL）
	DELAY_QUEUE = "order.delay.queue"
	// 死信队列（实际处理队列）
	DLQ_QUEUE = "order.dlq.queue"
	// 路由键
	DELAY_ROUTING_KEY = "order.delay"
	DLQ_ROUTING_KEY   = "order.dlq"

	// 库存事件交换机（发布取消回补事件）
	STOCK_EXCHANGE = "stock.topic.exchange"

	// 通知队列（发布订单取消/支付提醒通知事件）
	NOTIFY_QUEUE = "order.notification.queue"
)

// ==================== Prometheus 监控指标 ====================
var (
	// 延迟消息处理计数
	delayCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "delay_consumer_messages_total",
		Help: "Total number of delay messages processed",
	}, []string{"action", "status"})

	// 延迟消息处理耗时
	delayDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "delay_consumer_duration_seconds",
		Help:    "Duration of delay message processing",
		Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1, 5},
	}, []string{"action"})

	// 队列深度监控
	delayQueueGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "delay_consumer_queue_depth",
		Help: "Current depth of delay queue",
	})

	dlqQueueGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "delay_consumer_dlq_depth",
		Help: "Current depth of dead letter queue",
	})
)

func main() {
	var configFile = flag.String("f", "etc/delay-consumer.yaml", "配置文件路径")
	flag.Parse()

	var c DelayConfig
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

	// 初始化延迟队列架构
	if err := setupDelayQueue(rabbitMQ); err != nil {
		logx.Errorf("初始化延迟队列失败: %v", err)
		return
	}

	// 启动队列深度监控
	go monitorDelayQueueDepth(rabbitMQ)

	// 消费死信队列（实际处理队列）
	deliveryChan, err := rabbitMQ.Consume(
		DLQ_QUEUE,
		"order-delay-consumer",
		false, // 手动确认
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		logx.Errorf("启动延迟消费者失败: %v", err)
		return
	}

	logx.Info("订单延迟处理消费者启动成功...")

	// 处理延迟消息
	for msg := range deliveryChan {
		start := time.Now()
		if err := handleDelayMessage(ctx, redisClient, rabbitMQ, msg); err != nil {
			logx.Errorf("处理延迟消息失败: %v", err)
			delayCounter.WithLabelValues("unknown", "failed").Inc()
			msg.Nack(false, false) // 不重新入队
		} else {
			msg.Ack(false)
		}
		delayDuration.WithLabelValues("unknown").Observe(time.Since(start).Seconds())
	}
}

// monitorDelayQueueDepth 监控队列深度
func monitorDelayQueueDepth(r *mq.RabbitMQ) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		ch, err := r.GetChannel()
		if err != nil {
			continue
		}
		defer r.ReturnChannel(ch)

		// 监控延迟队列
		delayQueue, err := ch.QueueInspect(DELAY_QUEUE)
		if err == nil {
			delayQueueGauge.Set(float64(delayQueue.Messages))
		}

		// 监控死信队列
		dlqQueue, err := ch.QueueInspect(DLQ_QUEUE)
		if err == nil {
			dlqQueueGauge.Set(float64(dlqQueue.Messages))
		}
	}
}

// 初始化延迟队列架构
func setupDelayQueue(r *mq.RabbitMQ) error {
	ch, err := r.GetChannel()
	if err != nil {
		return err
	}
	defer r.ReturnChannel(ch)

	// 1. 声明死信交换机
	if err := ch.ExchangeDeclare(
		DLX_EXCHANGE,
		"direct",
		true,  // 持久化
		false, // 不自动删除
		false,
		false,
		nil,
	); err != nil {
		return fmt.Errorf("声明死信交换机失败: %w", err)
	}

	// 2. 声明延迟队列（30分钟TTL + 绑定死信交换机）
	_, err = ch.QueueDeclare(
		DELAY_QUEUE,
		true,
		false,
		false,
		false,
		amqp.Table{
			"x-dead-letter-exchange":    DLX_EXCHANGE,
			"x-dead-letter-routing-key": DLQ_ROUTING_KEY,
			"x-message-ttl":             30 * 60 * 1000, // 30分钟（毫秒）
		},
	)
	if err != nil {
		return fmt.Errorf("声明延迟队列失败: %w", err)
	}

	// 3. 声明死信队列（实际处理队列）
	_, err = ch.QueueDeclare(
		DLQ_QUEUE,
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("声明死信队列失败: %w", err)
	}

	// 4. 绑定死信队列到死信交换机
	if err := ch.QueueBind(
		DLQ_QUEUE,
		DLQ_ROUTING_KEY,
		DLX_EXCHANGE,
		false,
		nil,
	); err != nil {
		return fmt.Errorf("绑定死信队列失败: %w", err)
	}

	// 5. 声明库存 Topic 交换机（用于发布 order_cancel 回补事件）
	if err := ch.ExchangeDeclare(
		STOCK_EXCHANGE,
		"topic",
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		return fmt.Errorf("声明库存交换机失败: %w", err)
	}

	// 6. 声明通知队列（用于异步发送订单通知）
	_, err = ch.QueueDeclare(
		NOTIFY_QUEUE,
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("声明通知队列失败: %w", err)
	}

	logx.Info("延迟队列架构初始化完成")
	return nil
}

// handleDelayMessage 处理延迟消息
func handleDelayMessage(ctx context.Context, rdb *redis.Redis, rabbit *mq.RabbitMQ, msg amqp.Delivery) error {
	var delayMsg OrderDelayMessage
	if err := json.Unmarshal(msg.Body, &delayMsg); err != nil {
		delayCounter.WithLabelValues("parse", "failed").Inc()
		return fmt.Errorf("解析延迟消息失败: %w", err)
	}

	start := time.Now()
	defer func() {
		delayDuration.WithLabelValues(delayMsg.Action).Observe(time.Since(start).Seconds())
	}()

	logx.Infof("处理延迟消息: OrderID=%d, Action=%s, Delay=%v",
		delayMsg.OrderID, delayMsg.Action, time.Since(delayMsg.CreatedAt))

	// 幂等性检查 - 防止重复处理
	key := fmt.Sprintf("delay_event:%d:%s", delayMsg.OrderID, delayMsg.Action)
	exists, _ := rdb.Exists(key)
	if exists {
		logx.Infof("重复的延迟事件，跳过: OrderID=%d, Action=%s", delayMsg.OrderID, delayMsg.Action)
		delayCounter.WithLabelValues(delayMsg.Action, "skipped").Inc()
		return nil
	}
	// 设置处理标记（1小时后过期）
	rdb.Setex(key, "1", 3600)

	switch delayMsg.Action {
	case "cancel":
		err := handleOrderCancellation(ctx, rdb, rabbit, &delayMsg)
		if err != nil {
			delayCounter.WithLabelValues("cancel", "failed").Inc()
			return err
		}
		delayCounter.WithLabelValues("cancel", "success").Inc()
		return nil
	case "remind":
		err := handlePaymentReminder(ctx, rabbit, &delayMsg)
		if err != nil {
			delayCounter.WithLabelValues("remind", "failed").Inc()
			return err
		}
		delayCounter.WithLabelValues("remind", "success").Inc()
		return nil
	default:
		return fmt.Errorf("未知的延迟动作: %s", delayMsg.Action)
	}
}

// handleOrderCancellation 处理订单取消
func handleOrderCancellation(ctx context.Context, rdb *redis.Redis, rabbit *mq.RabbitMQ, msg *OrderDelayMessage) error {
	logx.Infof("处理订单取消: OrderID=%d", msg.OrderID)

	// TODO: 实际项目中调用订单服务检查状态并取消
	// 1. 调用订单 RPC 查询订单状态
	// order, err := orderClient.GetOrder(ctx, &order.GetOrderRequest{Id: msg.OrderID})
	// if err != nil {
	//     return fmt.Errorf("查询订单失败: %w", err)
	// }

	// 2. 检查订单状态（只有待支付状态才能取消）
	// if order.Status != OrderStatusPending {
	//     logx.Infof("订单状态不是待支付，跳过取消: OrderID=%d, Status=%d", msg.OrderID, order.Status)
	//     return nil
	// }

	// 3. 调用订单 RPC 取消订单
	// _, err = orderClient.CancelOrder(ctx, &order.CancelOrderRequest{Id: msg.OrderID})
	// if err != nil {
	//     return fmt.Errorf("取消订单失败: %w", err)
	// }

	items := msg.Items
	if len(items) == 0 {
		snapshotItems, err := loadOrderItemsSnapshot(rdb, msg.OrderID)
		if err != nil {
			logx.Errorf("加载订单商品快照失败: OrderID=%d, err=%v", msg.OrderID, err)
		} else {
			items = snapshotItems
		}
	}

	hasStockOp := len(items) > 0
	if hasStockOp {
		if err := publishStockRollbackEvents(rabbit, msg, items); err != nil {
			return fmt.Errorf("发布库存回补事件失败(OrderID=%d): %w", msg.OrderID, err)
		}
	} else {
		// 无商品快照时记录补偿任务，避免静默丢失库存回补
		recordMissingStockCompensation(rdb, msg)
		logx.Errorf("订单取消缺少商品快照，已记录补偿任务: OrderID=%d", msg.OrderID)
	}

	// 发送订单取消通知（异步投递）
	if err := sendOrderCancellationNotification(ctx, rabbit, msg, len(items), hasStockOp); err != nil {
		return fmt.Errorf("发送取消通知失败(OrderID=%d): %w", msg.OrderID, err)
	}

	logx.Infof("订单已取消: OrderID=%d, UserID=%d", msg.OrderID, msg.UserID)
	return nil
}

// handlePaymentReminder 处理支付提醒
func handlePaymentReminder(ctx context.Context, rabbit *mq.RabbitMQ, msg *OrderDelayMessage) error {
	logx.Infof("发送支付提醒: OrderID=%d, UserID=%d", msg.OrderID, msg.UserID)

	// TODO: 实际项目中调用通知服务
	// 1. 查询用户信息获取联系方式
	// user, err := userClient.GetUser(ctx, &user.GetUserRequest{Id: msg.UserID})

	// 2. 发送支付提醒通知
	// smsClient.Send(ctx, &sms.Request{
	//     Phone: user.Phone,
	//     Template: "payment_reminder",
	//     Data: map[string]string{"orderId": fmt.Sprintf("%d", msg.OrderID)},
	// })

	evt := OrderNotificationEvent{
		EventID:    fmt.Sprintf("notify:payment_reminder:%d", msg.OrderID),
		Type:       "payment_reminder",
		OrderID:    msg.OrderID,
		UserID:     msg.UserID,
		ItemCount:  len(msg.Items),
		HasStockOp: false,
		CreatedAt:  time.Now(),
	}
	if err := publishOrderNotificationEvent(rabbit, &evt); err != nil {
		return err
	}
	logx.Infof("支付提醒已入队: OrderID=%d, UserID=%d", msg.OrderID, msg.UserID)
	return nil
}

// sendOrderCancellationNotification 发送订单取消通知
func sendOrderCancellationNotification(ctx context.Context, rabbit *mq.RabbitMQ, msg *OrderDelayMessage, itemCount int, hasStockOp bool) error {
	logx.Infof("发送订单取消通知: OrderID=%d, UserID=%d", msg.OrderID, msg.UserID)

	evt := OrderNotificationEvent{
		EventID:    fmt.Sprintf("notify:order_cancelled:%d", msg.OrderID),
		Type:       "order_cancelled",
		OrderID:    msg.OrderID,
		UserID:     msg.UserID,
		ItemCount:  itemCount,
		HasStockOp: hasStockOp,
		CreatedAt:  time.Now(),
	}
	return publishOrderNotificationEvent(rabbit, &evt)
}

func publishStockRollbackEvents(rabbit *mq.RabbitMQ, msg *OrderDelayMessage, items []OrderItem) error {
	for _, item := range items {
		event := StockChangeEvent{
			EventID:   fmt.Sprintf("stock:order_cancel:%d:%d", msg.OrderID, item.ProductID),
			ProductID: item.ProductID,
			ChangeQty: item.Quantity,
			OrderID:   msg.OrderID,
			Reason:    "order_cancel",
			Timestamp: time.Now(),
		}
		body, err := json.Marshal(event)
		if err != nil {
			return fmt.Errorf("序列化库存事件失败(product_id=%d): %w", item.ProductID, err)
		}
		routingKey := "stock.change.order_cancel"
		if err := rabbit.Publish(STOCK_EXCHANGE, routingKey, false, false, amqp.Publishing{
			ContentType:  "application/json",
			Body:         body,
			DeliveryMode: amqp.Persistent,
			MessageId:    event.EventID,
			Timestamp:    event.Timestamp,
		}); err != nil {
			return err
		}
	}
	return nil
}

func publishOrderNotificationEvent(rabbit *mq.RabbitMQ, evt *OrderNotificationEvent) error {
	body, err := json.Marshal(evt)
	if err != nil {
		return err
	}
	return rabbit.Publish("", NOTIFY_QUEUE, false, false, amqp.Publishing{
		ContentType:  "application/json",
		Body:         body,
		DeliveryMode: amqp.Persistent,
		MessageId:    evt.EventID,
		Timestamp:    evt.CreatedAt,
	})
}

func loadOrderItemsSnapshot(rdb *redis.Redis, orderID int64) ([]OrderItem, error) {
	if rdb == nil {
		return nil, nil
	}
	key := fmt.Sprintf("order:items_snapshot:%d", orderID)
	raw, err := rdb.Get(key)
	if err != nil {
		return nil, err
	}
	if raw == "" {
		return nil, nil
	}
	var items []OrderItem
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return nil, err
	}
	return items, nil
}

func recordMissingStockCompensation(rdb *redis.Redis, msg *OrderDelayMessage) {
	if rdb == nil {
		return
	}
	payload, err := json.Marshal(map[string]interface{}{
		"order_id":    msg.OrderID,
		"user_id":     msg.UserID,
		"action":      msg.Action,
		"created_at":  msg.CreatedAt,
		"recorded_at": time.Now(),
		"reason":      "missing_order_items_snapshot",
	})
	if err != nil {
		logx.Errorf("序列化补偿记录失败: OrderID=%d, err=%v", msg.OrderID, err)
		return
	}
	key := fmt.Sprintf("order:stock_reconcile:%d", msg.OrderID)
	if err := rdb.Setex(key, string(payload), 7*24*3600); err != nil {
		logx.Errorf("写入补偿记录失败: key=%s, err=%v", key, err)
	}
}

// PublishDelayMessage 发布延迟消息的辅助函数（在订单服务中使用）
func PublishDelayMessage(r *mq.RabbitMQ, orderID, userID int64, action string) error {
	msg := OrderDelayMessage{
		OrderID:   orderID,
		UserID:    userID,
		Action:    action,
		CreatedAt: time.Now(),
	}

	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	return r.Publish("", DELAY_QUEUE, false, false, amqp.Publishing{
		ContentType:  "application/json",
		Body:         body,
		DeliveryMode: amqp.Persistent,
	})
}
