package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/wansui976/go_zero_shop/pkg/mq"
	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

// 库存变更事件
type StockChangeEvent struct {
	EventID   string    `json:"event_id"` // 事件唯一ID（幂等性）
	ProductID int64     `json:"product_id"`
	ChangeQty int64     `json:"change_qty"` // 正数=增加，负数=减少
	OrderID   int64     `json:"order_id"`
	Reason    string    `json:"reason"` // order_create, order_cancel, restock
	Timestamp time.Time `json:"timestamp"`
}

type StockConfig struct {
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
	STOCK_EXCHANGE    = "stock.topic.exchange"
	STOCK_QUEUE       = "stock.change.queue"
	STOCK_ROUTING_KEY = "stock.change.*"
)

// ==================== Prometheus 监控指标 ====================
var (
	// 消费消息数
	consumeCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "stock_consumer_messages_total",
		Help: "Total number of stock change messages consumed",
	}, []string{"reason", "status"})

	// 消费耗时
	consumeDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "stock_consumer_duration_seconds",
		Help:    "Duration of stock change message processing",
		Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1, 5},
	}, []string{"reason"})

	// 幂等性跳过数
	idempotentSkipped = promauto.NewCounter(prometheus.CounterOpts{
		Name: "stock_consumer_idempotent_skipped_total",
		Help: "Total number of duplicate events skipped by idempotency check",
	})

	// 队列堆积数
	queueGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "stock_consumer_queue_depth",
		Help: "Current depth of stock change queue",
	})
)

func main() {
	var configFile = flag.String("f", "etc/stock-consumer.yaml", "配置文件路径")
	flag.Parse()

	var c StockConfig
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

	// 初始化 Topic 交换机
	if err := setupStockExchange(rabbitMQ); err != nil {
		logx.Errorf("初始化库存交换机失败: %v", err)
		return
	}

	// 启动队列深度监控
	go monitorQueueDepth(rabbitMQ)

	// 设置消费者 QoS
	if err := rabbitMQ.Qos(5, 0, false); err != nil {
		logx.Errorf("设置QoS失败: %v", err)
		return
	}

	// 开始消费
	deliveryChan, err := rabbitMQ.Consume(
		STOCK_QUEUE,
		"stock-consumer",
		false, // 手动确认
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		logx.Errorf("启动库存消费者失败: %v", err)
		return
	}

	logx.Info("库存变更消费者启动成功...")

	// 处理消息
	for msg := range deliveryChan {
		start := time.Now()
		if err := handleStockChange(ctx, redisClient, msg); err != nil {
			logx.Errorf("处理库存变更失败: %v", err)
			consumeCounter.WithLabelValues("unknown", "failed").Inc()

			// 失败重试3次后进入死信队列
			var retryCount int32
			if retryVal, exists := msg.Headers["x-retry-count"]; exists {
				if count, ok := retryVal.(int32); ok {
					retryCount = count
				}
			}

			if retryCount < 3 {
				msg.Headers["x-retry-count"] = retryCount + 1
				msg.Nack(false, true) // 重新入队
			} else {
				msg.Nack(false, false) // 不重新入队，进入死信
				consumeCounter.WithLabelValues("unknown", "dlq").Inc()
			}
		} else {
			msg.Ack(false)
		}
		consumeDuration.WithLabelValues("unknown").Observe(time.Since(start).Seconds())
	}
}

// monitorQueueDepth 监控队列深度
func monitorQueueDepth(r *mq.RabbitMQ) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		ch, err := r.GetChannel()
		if err != nil {
			continue
		}
		defer r.ReturnChannel(ch)

		// 获取队列信息
		queue, err := ch.QueueInspect(STOCK_QUEUE)
		if err != nil {
			continue
		}
		queueGauge.Set(float64(queue.Messages))
	}
}

func setupStockExchange(r *mq.RabbitMQ) error {
	ch, err := r.GetChannel()
	if err != nil {
		return err
	}
	defer r.ReturnChannel(ch)

	// 声明 Topic 交换机
	if err := ch.ExchangeDeclare(
		STOCK_EXCHANGE,
		"topic",
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		return err
	}

	// 声明队列
	_, err = ch.QueueDeclare(
		STOCK_QUEUE,
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return err
	}

	// 绑定队列到交换机
	if err := ch.QueueBind(
		STOCK_QUEUE,
		STOCK_ROUTING_KEY,
		STOCK_EXCHANGE,
		false,
		nil,
	); err != nil {
		return err
	}

	logx.Info("库存Topic交换机初始化完成")
	return nil
}

// handleStockChange 处理库存变更（带幂等性检查）
func handleStockChange(ctx context.Context, rdb *redis.Redis, msg amqp.Delivery) error {
	var event StockChangeEvent
	if err := json.Unmarshal(msg.Body, &event); err != nil {
		consumeCounter.WithLabelValues("parse_error", "failed").Inc()
		return fmt.Errorf("解析库存变更事件失败: %w", err)
	}

	start := time.Now()
	defer func() {
		consumeDuration.WithLabelValues(event.Reason).Observe(time.Since(start).Seconds())
	}()

	// 幂等性检查（使用 Redis SETNX）
	if isDuplicate, err := checkDuplicate(ctx, rdb, event.EventID); err != nil {
		return fmt.Errorf("幂等性检查失败: %w", err)
	} else if isDuplicate {
		logx.Infof("重复的库存变更事件，跳过: EventID=%s", event.EventID)
		idempotentSkipped.Inc()
		consumeCounter.WithLabelValues(event.Reason, "skipped").Inc()
		return nil
	}

	logx.Infof("处理库存变更: ProductID=%d, ChangeQty=%d, Reason=%s",
		event.ProductID, event.ChangeQty, event.Reason)

	// 业务处理
	var err error
	switch event.Reason {
	case "order_create":
		err = handleOrderStockDecrease(ctx, &event)
	case "order_cancel":
		err = handleOrderStockIncrease(ctx, &event)
	case "restock":
		err = handleRestock(ctx, &event)
	default:
		err = fmt.Errorf("未知的库存变更原因: %s", event.Reason)
	}

	if err != nil {
		consumeCounter.WithLabelValues(event.Reason, "failed").Inc()
	} else {
		consumeCounter.WithLabelValues(event.Reason, "success").Inc()
	}

	return err
}

// checkDuplicate 幂等性检查（Redis GET/SET 实现）
// key 格式: "stock_event:{eventID}"
// TTL: 24小时
func checkDuplicate(ctx context.Context, rdb *redis.Redis, eventID string) (bool, error) {
	if eventID == "" {
		return false, nil
	}

	key := fmt.Sprintf("stock_event:%s", eventID)
	// 使用 GET 检查是否已存在
	val, _ := rdb.Get(key)
	if val == "1" {
		logx.Debugf("检测到重复事件: EventID=%s", eventID)
		return true, nil // 已存在，重复事件
	}

	// 设置新值（带过期时间）
	rdb.Setex(key, "1", 86400)
	return false, nil // 新事件
}

func handleOrderStockDecrease(ctx context.Context, event *StockChangeEvent) error {
	// 扣减库存（实际项目中应调用 product RPC）
	logx.Infof("扣减库存: ProductID=%d, Qty=%d, OrderID=%d",
		event.ProductID, event.ChangeQty, event.OrderID)

	// TODO: 调用 product RPC 更新库存
	// productClient.DecrStock(ctx, &product.DecrStockRequest{...})

	// TODO: 记录库存变更日志
	return nil
}

func handleOrderStockIncrease(ctx context.Context, event *StockChangeEvent) error {
	// 恢复库存
	logx.Infof("恢复库存: ProductID=%d, Qty=%d, OrderID=%d",
		event.ProductID, event.ChangeQty, event.OrderID)

	// TODO: 调用 product RPC 恢复库存
	return nil
}

func handleRestock(ctx context.Context, event *StockChangeEvent) error {
	// 补货入库
	logx.Infof("补货入库: ProductID=%d, Qty=%d", event.ProductID, event.ChangeQty)

	// TODO: 更新库存
	return nil
}

// PublishStockChangeEvent 发布库存变更事件（在 Product 服务中使用）
func PublishStockChangeEvent(r *mq.RabbitMQ, event *StockChangeEvent) error {
	body, err := json.Marshal(event)
	if err != nil {
		return err
	}

	routingKey := fmt.Sprintf("stock.change.%s", event.Reason)
	return r.Publish(STOCK_EXCHANGE, routingKey, false, false, amqp.Publishing{
		ContentType:  "application/json",
		Body:         body,
		DeliveryMode: amqp.Persistent,
		MessageId:    event.EventID,
		Timestamp:    time.Now(),
	})
}
