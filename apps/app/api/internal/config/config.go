package config

import (
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	rest.RestConf
	DtmServer  string
	OrderRPC   zrpc.RpcClientConf
	ProductRPC zrpc.RpcClientConf
	ReplyRPC   zrpc.RpcClientConf
	UserRPC    zrpc.RpcClientConf
	CartRPC    zrpc.RpcClientConf
	SearchRPC  zrpc.RpcClientConf
	PayRPC     zrpc.RpcClientConf

	JwtAuth struct {
		AccessSecret string
		AccessExpire int64
	}
	// Redis for business use (幂等/缓存等)
	BizRedis redis.RedisConf
}
