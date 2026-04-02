package svc

import (
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/wansui976/go_zero_shop/apps/pay/rpc/internal/config"
	"github.com/wansui976/go_zero_shop/apps/pay/rpc/model"
)

type ServiceContext struct {
	Config       config.Config
	PaymentModel model.PaymentModel
	RefundModel  model.RefundModel
}

func NewServiceContext(c config.Config) *ServiceContext {
	// 创建数据库连接
	conn := sqlx.NewMysql(c.DataSource)
	
	return &ServiceContext{
		Config:       c,
		PaymentModel: model.NewPaymentModel(conn, c.CacheRedis),
		RefundModel:  model.NewRefundModel(conn, c.CacheRedis),
	}
}
