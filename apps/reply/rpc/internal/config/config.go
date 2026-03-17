package config

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/zrpc"
)

type SnowflakeConfig struct {
	NodeID int64 `json:"NodeID"`
}

type Config struct {
	zrpc.RpcServerConf
	DataSource string
	CacheRedis cache.CacheConf
	Snowflake  SnowflakeConfig `json:"Snowflake"`
}
