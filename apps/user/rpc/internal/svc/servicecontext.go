package svc

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
	"github.com/wansui976/go_zero_shop/apps/user/rpc/internal/config"
	"github.com/wansui976/go_zero_shop/apps/user/rpc/model"
	"github.com/wansui976/go_zero_shop/pkg/envcfg"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type ServiceContext struct {
	Config                  config.Config
	UserModel               model.UserModel
	UserReceiveAddressModel model.UserReceiveAddressModel
	UserCollectionModel     model.UserCollectionModel
}

func NewServiceContext(c config.Config) *ServiceContext {
	// 敏感配置从 env 覆盖（K8s Secret 注入）
	envcfg.MustNonEmpty("MYSQL_PASSWORD")
	envcfg.MustNonEmpty("USER_JWT_SALT")
	c.Mysql.DataSource = envcfg.InjectMySQLDSNPassword(c.Mysql.DataSource, "MYSQL_PASSWORD")
	envcfg.OverrideCacheConf(c.CacheRedis, "REDIS_PASSWORD")
	if v := envcfg.Getenv("REDIS_PASSWORD", ""); v != "" {
		c.BizRedis.Pass = v
	}
	c.Salt = envcfg.OverrideString(c.Salt, "USER_JWT_SALT")

	sqlConn := sqlx.NewMysql(c.Mysql.DataSource)

	// Redis（用于业务版本号 INCR）
	rdb := redis.NewClient(&redis.Options{
		Addr:     c.BizRedis.Host,
		Password: c.BizRedis.Pass,
		DB:       c.BizRedis.DB,
	})
	// 验证 Redis 连接
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		fmt.Printf("warning: ping redis failed: %v\n", err)
	}

	return &ServiceContext{
		Config: c,

		UserModel: model.NewUserModel(
			sqlConn,
			c.CacheRedis, // ← 用你配置文件里的 CacheRedis
		),

		UserReceiveAddressModel: model.NewUserReceiveAddressModelWithRedis(
			sqlConn,
			c.CacheRedis, // ← 同样使用配置里的 CacheRedis
			rdb,
		),

		UserCollectionModel: model.NewUserCollectionModel(
			sqlConn,
			c.CacheRedis, // ← 同样使用配置里的 CacheRedis
		),
	}
}
