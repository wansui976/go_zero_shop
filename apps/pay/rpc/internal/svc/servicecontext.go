package svc

import (
	"github.com/bwmarrin/snowflake"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/wansui976/go_zero_shop/apps/pay/rpc/internal/config"
	"github.com/wansui976/go_zero_shop/apps/pay/rpc/model"
)

type ServiceContext struct {
	Config        config.Config
	PaymentModel  model.PaymentModel
	RefundModel   model.RefundModel
	SnowflakeNode *snowflake.Node
}

func NewServiceContext(c config.Config) *ServiceContext {
	conn := sqlx.NewMysql(c.DataSource)

	node, err := snowflake.NewNode(c.Snowflake.NodeID)
	if err != nil {
		logx.Errorf("failed to create snowflake node: %v", err)
	}

	return &ServiceContext{
		Config:        c,
		PaymentModel:  model.NewPaymentModel(conn, c.CacheRedis),
		RefundModel:   model.NewRefundModel(conn, c.CacheRedis),
		SnowflakeNode: node,
	}
}
