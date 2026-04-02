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
	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/sqlx"

	"github.com/wansui976/go_zero_shop/apps/pay/rpc/model"
)

type PayCallbackEvent struct {
	Event       string `json:"event"`
	PaymentID   string `json:"payment_id"`
	OrderID     string `json:"order_id"`
	Status      int    `json:"status"`       // 支付状态
	Amount      int64  `json:"amount"`       // 支付金额
	TransactionID string `json:"transaction_id"` // 第三方交易号
	Time        time.Time `json:"time"`
}

type PayConfig struct {
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
	DataSource  string
	CacheRedis  cache.CacheConf
}

const (
	PAY_CALLBACK_QUEUE = "pay_callback_queue"
	PAY_EXCHANGE       = "pay_exchange"
	PAY_ROUTING_KEY    = "pay.callback"
)

var (
	payConsumeCounter = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "pay_consumer_messages_total",
		Help: "Total number of pay callback messages consumed",
	}, []string{"action", "status"})

	payConsumeDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "pay_consumer_duration_seconds",
		Help:    "Duration of pay callback message processing",
		Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1, 5},
	})

	payQueueGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "pay_consumer_queue_depth",
		Help: "Current depth of pay callback queue",
	})
)

func main() {
	var configFile = flag.String("f", "etc/pay-callback.yaml", "配置文件路径")
	flag.Parse()

	var c PayConfig
	conf.MustLoad(*configFile, &c)

	ctx := context.Background()

	if err := consume(ctx, c); err != nil {
		logx.Errorf("consumer error: %v", err)
	}
}

func consume(ctx context.Context, c PayConfig) error {
	// 连接 RabbitMQ
	conn, err := amqp.Dial(c.RabbitMQ.URL)
	if err != nil {
		logx.Errorf("failed to connect to RabbitMQ: %v", err)
		return err
	}
	defer conn.Close()

	// 创建 Channel
	channel, err := conn.Channel()
	if err != nil {
		logx.Errorf("failed to open channel: %v", err)
		return err
	}
	defer channel.Close()

	// 声明 Exchange
	err = channel.ExchangeDeclare(
		PAY_EXCHANGE, // name
		"direct",     // type
		true,         // durable
		false,        // auto-deleted
		false,        // internal
		false,        // no-wait
		nil,          // arguments
	)
	if err != nil {
		logx.Errorf("failed to declare exchange: %v", err)
		return err
	}

	// 声明 Queue
	q, err := channel.QueueDeclare(
		PAY_CALLBACK_QUEUE, // name
		true,              // durable
		false,             // delete when unused
		false,             // exclusive
		false,             // no-wait
		nil,               // arguments
	)
	if err != nil {
		logx.Errorf("failed to declare queue: %v", err)
		return err
	}

	// 绑定 Queue 到 Exchange
	err = channel.QueueBind(
		q.Name,            // queue name
		PAY_ROUTING_KEY,  // routing key
		PAY_EXCHANGE,     // exchange
		false,
		nil,
	)
	if err != nil {
		logx.Errorf("failed to bind queue: %v", err)
		return err
	}

	// 设置 Prefetch
	err = channel.Qos(
		c.RabbitMQ.PoolSize, // prefetch count
		0,                   // prefetch size
		false,               // global
	)
	if err != nil {
		logx.Errorf("failed to set QoS: %v", err)
		return err
	}

	// 连接数据库
	connDB := sqlx.NewMysql(c.DataSource)
	paymentModel := model.NewPaymentModel(connDB, c.CacheRedis)

	logx.Info("Pay callback consumer started, waiting for messages...")

	// 消费消息
	deliveries, err := channel.Consume(
		q.Name, // queue
		"",     // consumer
		false,  // auto-ack
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	if err != nil {
		logx.Errorf("failed to register consumer: %v", err)
		return err
	}

	// 监控队列深度
	go monitorQueueDepth(channel, q.Name)

	for {
		select {
		case <-ctx.Done():
			return nil
		case msg, ok := <-deliveries:
			if !ok {
				logx.Errorf("channel closed")
				return fmt.Errorf("channel closed")
			}

			startTime := time.Now()
			err := processMessage(ctx, msg.Body, paymentModel)
			if err != nil {
				logx.Errorf("failed to process message: %v", err)
				msg.Nack(false, true) // requeue
				payConsumeCounter.WithLabelValues("process", "failed").Inc()
			} else {
				msg.Ack(false)
				payConsumeCounter.WithLabelValues("process", "success").Inc()
			}

			duration := time.Since(startTime).Seconds()
			payConsumeDuration.Observe(duration)
			logx.Infof("message processed in %.3f seconds", duration)
		}
	}
}

func processMessage(ctx context.Context, body []byte, paymentModel model.PaymentModel) error {
	var event PayCallbackEvent
	err := json.Unmarshal(body, &event)
	if err != nil {
		logx.Errorf("failed to unmarshal message: %v", err)
		return err
	}

	logx.Infof("received pay callback event: payment_id=%s, order_id=%s, status=%d", 
		event.PaymentID, event.OrderID, event.Status)

	// 查询支付单
	payment, err := paymentModel.FindOneByPaymentId(ctx, event.PaymentID)
	if err != nil {
		logx.Errorf("payment not found: %s, err: %v", event.PaymentID, err)
		return err
	}

	// 更新支付状态
	payment.Status = event.Status
	payment.TransactionId = event.TransactionID
	payment.PayTime = time.Now().Unix()

	err = paymentModel.Update(ctx, payment)
	if err != nil {
		logx.Errorf("failed to update payment: %v", err)
		return err
	}

	logx.Infof("payment updated: payment_id=%s, status=%d", event.PaymentID, event.Status)
	return nil
}

func monitorQueueDepth(channel *amqp.Channel, queueName string) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		q, err := channel.QueueInspect(queueName)
		if err != nil {
			continue
		}
		payQueueGauge.Set(float64(q.Messages))
	}
}
