package config

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	zrpc.RpcServerConf
	DataSource string
	CacheRedis cache.CacheConf
	BizRedis   redis.RedisConf
	AsynqRedis AsynqRedisConf // 注意：类型是自定义的AsynqRedisConf（含DB字段）
}

type AsynqRedisConf struct {
	Host string `json:"Host"` // Redis地址（如127.0.0.1:6379）
	Pass string `json:"Pass"` // Redis密码（留空表示无密码）
	Type string `json:"Type"` // 节点类型（node=单机，cluster=集群）
	DB   int    `json:"DB"`   // Redis数据库编号（对应yaml中的AsynqRedis.DB）
}
