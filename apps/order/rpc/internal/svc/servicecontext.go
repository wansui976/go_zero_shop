package svc

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/wansui976/go_zero_shop/apps/order/rpc/internal/config"
	"github.com/wansui976/go_zero_shop/apps/order/rpc/model"
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
	//声明订单实践队列
	orderEventQueue := "order_create_queue"
	err = rabbitMQ.DeclareQueue(orderEventQueue,
		true,  // 队列持久化
		false, // 不自动删除
		false, // 非排他性
		false, // 等待响应
		nil,
	)
	if err != nil {
		panic(fmt.Errorf("声明订单队列失败: %v", err))

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
