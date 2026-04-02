package config

import (
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/zrpc"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

type Config struct {
	rest.RestConf
	DtmServer         string
	OrderRPC          zrpc.RpcClientConf
	ProductRPC        zrpc.RpcClientConf
	ReplyRPC          zrpc.RpcClientConf
	UserRPC           zrpc.RpcClientConf
	CartRPC           zrpc.RpcClientConf
	SearchRPC         zrpc.RpcClientConf
	// DTM Saga 直连地址（DTM server 回调时使用，需与各 RPC 服务 ListenOn 一致）
	OrderServiceAddr   string // e.g. "127.0.0.1:8082"
	ProductServiceAddr string // e.g. "127.0.0.1:8081"

	JwtAuth struct {
		AccessSecret string
		AccessExpire int64
	}
	// Redis for business use (幂等/缓存等)
	BizRedis redis.RedisConf
}
