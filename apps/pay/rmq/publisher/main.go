package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
)

type PayCallbackEvent struct {
	Event         string    `json:"event"`
	PaymentID     string    `json:"payment_id"`
	OrderID       string    `json:"order_id"`
	Status        int       `json:"status"`
	Amount        int64     `json:"amount"`
	TransactionID string    `json:"transaction_id"`
	Time          time.Time `json:"time"`
}

type PublisherConfig struct {
	RabbitMQ struct {
		URL string `json:"Url"`
	}
}

const (
	PAY_EXCHANGE    = "pay_exchange"
	PAY_ROUTING_KEY = "pay.callback"
)

func main() {
	var configFile = flag.String("f", "etc/pay-publisher.yaml", "配置文件路径")
	var paymentID = flag.String("payment-id", "", "支付单号")
	var orderID = flag.String("order-id", "", "订单号")
	var status = flag.Int("status", 1, "支付状态")
	var amount = flag.Int64("amount", 0, "支付金额")
	flag.Parse()

	if *paymentID == "" || *orderID == "" {
		fmt.Println("Usage: pay-publisher -f config.yaml -payment-id <id> -order-id <id> -status <status>")
		return
	}

	var c PublisherConfig
	conf.MustLoad(*configFile, &c)

	// 连接 RabbitMQ
	conn, err := amqp.Dial(c.RabbitMQ.URL)
	if err != nil {
		log.Fatalf("failed to connect to RabbitMQ: %v", err)
	}
	defer conn.Close()

	channel, err := conn.Channel()
	if err != nil {
		log.Fatalf("failed to open channel: %v", err)
	}
	defer channel.Close()

	// 声明 Exchange
	err = channel.ExchangeDeclare(
		PAY_EXCHANGE,
		"direct",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		log.Fatalf("failed to declare exchange: %v", err)
	}

	// 构建消息
	event := PayCallbackEvent{
		Event:         "pay.callback",
		PaymentID:     *paymentID,
		OrderID:       *orderID,
		Status:        *status,
		Amount:        *amount,
		TransactionID: fmt.Sprintf("TXN_%d", time.Now().Unix()),
		Time:          time.Now(),
	}

	body, err := json.Marshal(event)
	if err != nil {
		log.Fatalf("failed to marshal event: %v", err)
	}

	// 发布消息
	err = channel.Publish(
		PAY_EXCHANGE,
		PAY_ROUTING_KEY,
		false,
		false,
		amqp.Publishing{
			ContentType:  "application/json",
			Body:         body,
			DeliveryMode: amqp.Persistent,
			Timestamp:    time.Now(),
		},
	)
	if err != nil {
		log.Fatalf("failed to publish message: %v", err)
	}

	logx.Infof("Published pay callback event: payment_id=%s, order_id=%s, status=%d",
		*paymentID, *orderID, *status)
}
