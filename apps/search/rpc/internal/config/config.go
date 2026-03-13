package config

import (
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	zrpc.RpcServerConf
	ProductRpc zrpc.RpcClientConf

	Es struct {
		Host     string
		Username string
		Password string
	}

	// 缓存配置
	CacheRedis redis.RedisConf
}
