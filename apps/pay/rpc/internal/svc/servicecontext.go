package svc

import (
	"github.com/bwmarrin/snowflake"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/wansui976/go_zero_shop/apps/pay/rpc/internal/config"
	"github.com/wansui976/go_zero_shop/apps/pay/rpc/model"
	"github.com/wansui976/go_zero_shop/pkg/envcfg"
	pkgsnowflake "github.com/wansui976/go_zero_shop/pkg/snowflake"
)

type ServiceContext struct {
	Config        config.Config
	PaymentModel  model.PaymentModel
	RefundModel   model.RefundModel
	SnowflakeNode *snowflake.Node
}

func NewServiceContext(c config.Config) *ServiceContext {
	envcfg.MustNonEmpty("MYSQL_PASSWORD")
	envcfg.MustNonEmpty("PAY_WEBHOOK_SECRET")
	c.DataSource = envcfg.InjectMySQLDSNPassword(c.DataSource, "MYSQL_PASSWORD")
	envcfg.OverrideCacheConf(c.CacheRedis, "REDIS_PASSWORD")
	envcfg.OverrideRedisConf(&c.BizRedis, "REDIS_PASSWORD")
	c.PayWebhookSecret = envcfg.OverrideString(c.PayWebhookSecret, "PAY_WEBHOOK_SECRET")

	conn := sqlx.NewMysql(c.DataSource)

	nodeID := pkgsnowflake.ResolveNodeID(c.Snowflake.NodeID)
	node, err := snowflake.NewNode(nodeID)
	if err != nil {
		logx.Errorf("failed to create snowflake node (id=%d): %v", nodeID, err)
	}

	return &ServiceContext{
		Config:        c,
		PaymentModel:  model.NewPaymentModel(conn, c.CacheRedis),
		RefundModel:   model.NewRefundModel(conn, c.CacheRedis),
		SnowflakeNode: node,
	}
}
