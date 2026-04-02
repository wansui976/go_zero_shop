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

// OrderNotificationEvent 延迟消费者投递的通知事件
type OrderNotificationEvent struct {
	EventID    string    `json:"event_id"`
	Type       string    `json:"type"` // order_cancelled, payment_reminder
	OrderID    int64     `json:"order_id"`
	UserID     int64     `json:"user_id"`
	ItemCount  int       `json:"item_count"`
	HasStockOp bool      `json:"has_stock_op"`
	CreatedAt  time.Time `json:"created_at"`
}

type NotifyConfig struct {
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
	NOTIFY_QUEUE = "order.notification.queue"
)

var (
	notifyCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "notify_consumer_messages_total",
		Help: "Total number of notification messages consumed",
	}, []string{"type", "status"})

	notifyDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "notify_consumer_duration_seconds",
		Help:    "Duration of notification message processing",
		Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1, 5},
	}, []string{"type"})

	notifyQueueGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "notify_consumer_queue_depth",
		Help: "Current depth of notification queue",
	})
)

func main() {
	var configFile = flag.String("f", "etc/notify-consumer.yaml", "配置文件路径")
	flag.Parse()

	var c NotifyConfig
	conf.MustLoad(*configFile, &c)

	ctx := context.Background()

	rabbitMQ, err := mq.NewRabbitMQ(mq.RabbitMQConfig{
		URL:      c.RabbitMQ.URL,
		PoolSize: c.RabbitMQ.PoolSize,
	})
	if err != nil {
		logx.Errorf("初始化RabbitMQ失败: %v", err)
		return
	}
	defer rabbitMQ.Close()

	rdb, err := redis.NewRedis(redis.RedisConf{
		Host: c.Redis.Host,
		Type: c.Redis.Type,
		Pass: c.Redis.Pass,
	})
	if err != nil {
		logx.Errorf("初始化Redis失败: %v", err)
		return
	}

	if err := setupNotifyQueue(rabbitMQ); err != nil {
		logx.Errorf("初始化通知队列失败: %v", err)
		return
	}

	go monitorNotifyQueueDepth(rabbitMQ)

	deliveryChan, err := rabbitMQ.Consume(
		NOTIFY_QUEUE,
		"order-notify-consumer",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		logx.Errorf("启动通知消费者失败: %v", err)
		return
	}

	logx.Info("通知消费者启动成功...")

	for msg := range deliveryChan {
		start := time.Now()
		if err := handleNotification(ctx, rdb, msg); err != nil {
			logx.Errorf("处理通知消息失败: %v", err)
			notifyCounter.WithLabelValues("unknown", "failed").Inc()
			msg.Nack(false, false)
		} else {
			msg.Ack(false)
		}
		notifyDuration.WithLabelValues("unknown").Observe(time.Since(start).Seconds())
	}
}

func setupNotifyQueue(r *mq.RabbitMQ) error {
	ch, err := r.GetChannel()
	if err != nil {
		return err
	}
	defer r.ReturnChannel(ch)

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

	return nil
}

func monitorNotifyQueueDepth(r *mq.RabbitMQ) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		ch, err := r.GetChannel()
		if err != nil {
			continue
		}
		defer r.ReturnChannel(ch)

		queue, err := ch.QueueInspect(NOTIFY_QUEUE)
		if err != nil {
			continue
		}
		notifyQueueGauge.Set(float64(queue.Messages))
	}
}

func handleNotification(ctx context.Context, rdb *redis.Redis, msg amqp.Delivery) error {
	var evt OrderNotificationEvent
	if err := json.Unmarshal(msg.Body, &evt); err != nil {
		notifyCounter.WithLabelValues("parse", "failed").Inc()
		return fmt.Errorf("解析通知消息失败: %w", err)
	}

	start := time.Now()
	defer func() {
		notifyDuration.WithLabelValues(evt.Type).Observe(time.Since(start).Seconds())
	}()

	if isDuplicate := checkDuplicateEvent(rdb, evt.EventID); isDuplicate {
		logx.Infof("重复通知事件，跳过: EventID=%s", evt.EventID)
		notifyCounter.WithLabelValues(evt.Type, "skipped").Inc()
		return nil
	}

	switch evt.Type {
	case "order_cancelled":
		if err := processOrderCancelled(ctx, &evt); err != nil {
			notifyCounter.WithLabelValues(evt.Type, "failed").Inc()
			return err
		}
	case "payment_reminder":
		if err := processPaymentReminder(ctx, &evt); err != nil {
			notifyCounter.WithLabelValues(evt.Type, "failed").Inc()
			return err
		}
	default:
		notifyCounter.WithLabelValues("unknown", "failed").Inc()
		return fmt.Errorf("未知通知类型: %s", evt.Type)
	}

	notifyCounter.WithLabelValues(evt.Type, "success").Inc()
	return nil
}

func checkDuplicateEvent(rdb *redis.Redis, eventID string) bool {
	if rdb == nil || eventID == "" {
		return false
	}
	key := fmt.Sprintf("notify_event:%s", eventID)
	exists, err := rdb.Exists(key)
	if err != nil {
		logx.Errorf("通知幂等检查失败: eventID=%s, err=%v", eventID, err)
		return false
	}
	if exists {
		return true
	}
	if err := rdb.Setex(key, "1", 24*3600); err != nil {
		logx.Errorf("写入通知幂等键失败: key=%s, err=%v", key, err)
	}
	return false
}

func processOrderCancelled(ctx context.Context, evt *OrderNotificationEvent) error {
	logx.Infof("处理订单取消通知: OrderID=%d, UserID=%d, Items=%d, HasStockOp=%v",
		evt.OrderID, evt.UserID, evt.ItemCount, evt.HasStockOp)

	// TODO: 接入真实通知服务（短信/站内信/推送）
	return nil
}

func processPaymentReminder(ctx context.Context, evt *OrderNotificationEvent) error {
	logx.Infof("处理支付提醒通知: OrderID=%d, UserID=%d, Items=%d",
		evt.OrderID, evt.UserID, evt.ItemCount)

	// TODO: 接入真实通知服务（短信/站内信/推送）
	return nil
}
