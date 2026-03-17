package svc

import (
	"strings"

	"github.com/wansui976/go_zero_shop/apps/reply/rpc/internal/config"
	"github.com/wansui976/go_zero_shop/apps/reply/rpc/model"
	"github.com/wansui976/go_zero_shop/pkg/snowflake"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type ServiceContext struct {
	Config       config.Config
	CommentModel model.CommentModel
}

func NewServiceContext(c config.Config) *ServiceContext {
	if err := snowflake.Init(c.Snowflake.NodeID); err != nil && !strings.Contains(err.Error(), "已经初始化过了") {
		panic("snowflake 初始化失败: " + err.Error())
	}

	sqlConn := sqlx.NewMysql(c.DataSource)

	return &ServiceContext{
		Config:       c,
		CommentModel: model.NewCommentModel(sqlConn, c.CacheRedis),
	}
}
