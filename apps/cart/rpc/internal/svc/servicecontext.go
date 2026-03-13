package svc

import (
	"database/sql"
	"fmt"

	"github.com/wansui976/go_zero_shop/apps/cart/rpc/internal/config"
	"github.com/wansui976/go_zero_shop/apps/cart/rpc/model"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type ServiceContext struct {
	Config    config.Config
	DB        *sql.DB
	CartModel model.CartModel
}

func NewServiceContext(c config.Config) *ServiceContext {
	db, err := sqlx.NewMysql(c.DataSource).RawDB()
	if err != nil {
		panic(fmt.Sprintf("init db dailed:%v", err))
	}
	conn := sqlx.NewMysql(c.DataSource)
	return &ServiceContext{
		Config:    c,
		DB:        db,
		CartModel: model.NewCartModel(conn, c.CacheRedis),
	}
}
