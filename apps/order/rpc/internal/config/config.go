package config

import (
	//"github.com/segmentio/kafka-go"
	//"github.com/zeromicro/go-queue/kq"
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/zrpc"
)

type KafkaConfig struct {
	Brokers []string
	Topic   string
}

type Config struct {
	zrpc.RpcServerConf
	DataSource  string
	CacheRedis  cache.CacheConf
	ProductRpc  zrpc.RpcClientConf
	UserRpc     zrpc.RpcClientConf
	RabbitMQurl string
	Snowflake   struct {
		NodeID int64 `json:"NodeID"` // 雪花节点ID（0-1023）
	} `json:"Snowflake"`
	BizRedis redis.RedisConf
}
