package svc

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
	"github.com/wansui976/go_zero_shop/apps/app/api/internal/config"
	"github.com/wansui976/go_zero_shop/apps/cart/rpc/cart"
	"github.com/wansui976/go_zero_shop/apps/order/rpc/order"
	"github.com/wansui976/go_zero_shop/apps/pay/rpc/payclient"
	"github.com/wansui976/go_zero_shop/apps/product/rpc/product"
	"github.com/wansui976/go_zero_shop/apps/search/rpc/search"
	"github.com/wansui976/go_zero_shop/pkg/idempotent"

	//"github.com/wansui976/go_zero_shop/apps/reply/rpc/reply"
	"github.com/wansui976/go_zero_shop/apps/user/rpc/user"
	"github.com/zeromicro/go-zero/zrpc"
)

type ServiceContext struct {
	Config     config.Config
	OrderRPC   order.OrderServiceClient
	ProductRPC product.ProductClient
	//ReplyRPC   reply.ReplyClient
	UserRPC   user.UserClient
	CartRPC   cart.CartClient
	SearchRPC search.SearchClient
	PayRPC    payclient.Pay
	// Redis client for business uses (幂等/映射)
	Rdb   *redis.Client
	Idemp *idempotent.Idempotent
}

func NewServiceContext(c config.Config) *ServiceContext {
	// 初始化 Redis 客户端（用于幂等等业务场景）
	rdb := redis.NewClient(&redis.Options{
		Addr:     c.BizRedis.Host,
		Password: c.BizRedis.Pass,
		DB:       0,
	})

	// 简单 Ping 验证
	if err := rdb.Ping(ctxBackground()).Err(); err != nil {
		// 不要 panic，记录/打印以便运维发现；仍然返回带 nil 的 Idemp，调用方需注意空指针
		fmt.Printf("warning: ping redis failed: %v\n", err)
	}

	idemp := idempotent.NewIdempotent(rdb)

	return &ServiceContext{
		Config:     c,
		OrderRPC:   order.NewOrderServiceClient(zrpc.MustNewClient(c.OrderRPC).Conn()),
		ProductRPC: product.NewProductClient(zrpc.MustNewClient(c.ProductRPC).Conn()),
		//ReplyRPC:   reply.NewReplyClient(zrpc.MustNewClient(c.ReplyRPC).Conn()),
		SearchRPC: search.NewSearchClient(zrpc.MustNewClient(c.SearchRPC).Conn()),
		PayRPC:    payclient.NewPay(zrpc.MustNewClient(c.PayRPC)),
		UserRPC:   user.NewUserClient(zrpc.MustNewClient(c.UserRPC).Conn()),
		CartRPC:   cart.NewCartClient(zrpc.MustNewClient(c.CartRPC).Conn()),
		Rdb:       rdb,
		Idemp:     idemp,
	}
}

// helper: background context for init ping (avoids importing context in many files)
func ctxBackground() context.Context {
	return context.Background()
}
