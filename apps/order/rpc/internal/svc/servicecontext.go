package svc

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/redis/go-redis/v9"
	"github.com/wansui976/go_zero_shop/apps/order/rpc/internal/config"
	"github.com/wansui976/go_zero_shop/apps/order/rpc/model"
	"github.com/wansui976/go_zero_shop/apps/pay/rpc/payclient"
	"github.com/wansui976/go_zero_shop/apps/product/rpc/product"
	"github.com/wansui976/go_zero_shop/apps/product/rpc/productclient"
	"github.com/wansui976/go_zero_shop/apps/user/rpc/userclient"
	"github.com/wansui976/go_zero_shop/pkg/mq"
	"github.com/wansui976/go_zero_shop/pkg/snowflake"

	//"github.com/zeromicro/go-queue/kq"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zeromicro/go-zero/zrpc"
)

type ServiceContext struct {
	Config         config.Config
	DB             *sql.DB
	OrderModel     model.OrdersModel
	OrderitemModel model.OrderItemsModel
	ShippingModel  model.OrderAddressSnapshotModel
	UserRpc        userclient.User
	ProductRpc     product.ProductClient
	PayRpc         payclient.Pay
	//ProductModel
	//KafkaPusher    *kq.Pusher
	RabbitMQ *mq.RabbitMQ
	// Redis，用于读取 gid -> requestId 映射
	Rdb *redis.Client
}

func NewServiceContext(c config.Config) *ServiceContext {
	db, err := sqlx.NewMysql(c.DataSource).RawDB()
	if err != nil {
		panic(fmt.Sprintf("init db dailed:%v", err))
	}
	if err := snowflake.Init(1); err != nil {
		panic("snowflake 初始化失败: " + err.Error())
	}

	mqCfg := mq.RabbitMQConfig{
		URL:          c.RabbitMQurl,
		Heartbeat:    5 * time.Second,
		PoolSize:     10,              // 信道池大小（根据并发调整）
		MaxReconnect: 10,              // 最大重连次数
		ReconnectInt: 3 * time.Second, // 重连间隔
	}
	rabbitMQ, err := mq.NewRabbitMQ(mqCfg)
	if err != nil {
		panic(fmt.Errorf("初始化 MQ 客户端失败: %v", err))
	}
	for _, queueName := range []string{"order_create_queue", "order.notification.queue"} {
		err = rabbitMQ.DeclareQueue(queueName,
			true,
			false,
			false,
			false,
			nil,
		)
		if err != nil {
			panic(fmt.Errorf("声明队列失败(%s): %v", queueName, err))
		}
	}

	// 预先声明延迟处理基础设施，保证下单主链能够直接挂上延迟取消/提醒。
	ch, err := rabbitMQ.GetChannel()
	if err != nil {
		panic(fmt.Errorf("获取MQ信道失败: %v", err))
	}
	defer rabbitMQ.ReturnChannel(ch)
	if err := ch.ExchangeDeclare("dlx.exchange", "direct", true, false, false, false, nil); err != nil {
		panic(fmt.Errorf("声明死信交换机失败: %v", err))
	}
	if _, err := ch.QueueDeclare("order.delay.queue", true, false, false, false, amqp.Table{
		"x-dead-letter-exchange":    "dlx.exchange",
		"x-dead-letter-routing-key": "order.dlq",
		"x-message-ttl":             30 * 60 * 1000,
	}); err != nil {
		panic(fmt.Errorf("声明延迟队列失败: %v", err))
	}
	if _, err := ch.QueueDeclare("order.dlq.queue", true, false, false, false, nil); err != nil {
		panic(fmt.Errorf("声明订单死信队列失败: %v", err))
	}
	if err := ch.QueueBind("order.dlq.queue", "order.dlq", "dlx.exchange", false, nil); err != nil {
		panic(fmt.Errorf("绑定订单死信队列失败: %v", err))
	}
	conn := sqlx.NewMysql(c.DataSource)

	// init redis client for gid->request lookup
	rdb := redis.NewClient(&redis.Options{
		Addr:     c.BizRedis.Host,
		Password: c.BizRedis.Pass,
		DB:       0,
	})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		fmt.Printf("warning: ping redis failed: %v\n", err)
	}

	//kafkaPusher := kq.NewPusher(c.KafkaPusher.Brokers, c.KafkaPusher.Topic)
	return &ServiceContext{
		Config:         c,
		DB:             db,
		OrderModel:     model.NewOrdersModel(conn, c.CacheRedis),
		OrderitemModel: model.NewOrderItemsModel(conn, c.CacheRedis),
		ShippingModel:  model.NewOrderAddressSnapshotModel(conn, c.CacheRedis),
		UserRpc:        userclient.NewUser(zrpc.MustNewClient(c.UserRpc)),
		ProductRpc:     productclient.NewProduct(zrpc.MustNewClient(c.ProductRpc)),
		PayRpc:         payclient.NewPay(zrpc.MustNewClient(c.PayRpc)),
		//KafkaPusher:    kafkaPusher,
		RabbitMQ: rabbitMQ,
		Rdb:      rdb,
	}
}

func (s *ServiceContext) Close() {
	if s == nil {
		return
	}
	if s.RabbitMQ != nil {
		s.RabbitMQ.Close()
	}
	if s.Rdb != nil {
		_ = s.Rdb.Close()
	}
	if s.DB != nil {
		_ = s.DB.Close()
	}
}
