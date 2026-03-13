package config

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/zrpc"
)

type RedisConf struct {
	Host string `json:"host"`
	Pass string `json:"pass"`
	DB   int    `json:"db"`
}

type SnowflakeConfig struct {
	NodeID int64 `json:"NodeID"` // 雪花算法节点ID（0-1023）
}

type Config struct {
	zrpc.RpcServerConf
	Mysql struct {
		DataSource string
	}
	Snowflake  SnowflakeConfig `json:"Snowflake"`
	CacheRedis cache.CacheConf
	BizRedis   RedisConf
	Salt       string
}
