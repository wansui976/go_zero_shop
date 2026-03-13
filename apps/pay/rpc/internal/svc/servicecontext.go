package svc

import (
	"github.com/redis/go-redis/v9"
	orderModel "github.com/wansui976/go_zero_shop/apps/order/rpc/model"
	"github.com/wansui976/go_zero_shop/apps/pay/rpc/internal/config"
	"github.com/wansui976/go_zero_shop/apps/pay/rpc/model"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type ServiceContext struct {
	Config       config.Config
	PaymentModel model.PaymentModel
	RefundModel  model.RefundModel
	OrderModel   orderModel.OrdersModel
	BizRdb       *redis.Client
}

func NewServiceContext(c config.Config) *ServiceContext {
	conn := sqlx.NewMysql(c.DataSource)
	rdb := redis.NewClient(&redis.Options{
		Addr:     c.BizRedis.Host,
		Password: c.BizRedis.Pass,
		DB:       0,
	})

	return &ServiceContext{
		Config:       c,
		PaymentModel: model.NewPaymentModel(conn, c.CacheRedis),
		RefundModel:  model.NewRefundModel(conn, c.CacheRedis),
		OrderModel:   orderModel.NewOrdersModel(conn, c.CacheRedis),
		BizRdb:       rdb,
	}
}
