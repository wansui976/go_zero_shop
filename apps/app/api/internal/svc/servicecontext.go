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
	replyclient "github.com/wansui976/go_zero_shop/apps/reply/rpc/replyclient"
	"github.com/wansui976/go_zero_shop/apps/search/rpc/search"
	"github.com/wansui976/go_zero_shop/apps/user/rpc/user"
	"github.com/wansui976/go_zero_shop/pkg/idempotent"
	"github.com/zeromicro/go-zero/zrpc"
)

type ServiceContext struct {
	Config     config.Config
	OrderRPC   order.OrderServiceClient
	ProductRPC product.ProductClient
	ReplyRPC   replyclient.Reply
	UserRPC    user.UserClient
	CartRPC    cart.CartClient
	SearchRPC  search.SearchClient
	PayRPC     payclient.Pay
	Rdb        *redis.Client
	Idemp      *idempotent.Idempotent
}

func NewServiceContext(c config.Config) *ServiceContext {
	rdb := redis.NewClient(&redis.Options{
		Addr:     c.BizRedis.Host,
		Password: c.BizRedis.Pass,
		DB:       0,
	})

	if err := rdb.Ping(ctxBackground()).Err(); err != nil {
		fmt.Printf("warning: ping redis failed: %v\n", err)
	}

	idemp := idempotent.NewIdempotent(rdb)

	return &ServiceContext{
		Config:     c,
		OrderRPC:   order.NewOrderServiceClient(zrpc.MustNewClient(c.OrderRPC).Conn()),
		ProductRPC: product.NewProductClient(zrpc.MustNewClient(c.ProductRPC).Conn()),
		ReplyRPC:   replyclient.NewReply(zrpc.MustNewClient(c.ReplyRPC)),
		SearchRPC:  search.NewSearchClient(zrpc.MustNewClient(c.SearchRPC).Conn()),
		PayRPC:     payclient.NewPay(zrpc.MustNewClient(c.PayRPC)),
		UserRPC:    user.NewUserClient(zrpc.MustNewClient(c.UserRPC).Conn()),
		CartRPC:    cart.NewCartClient(zrpc.MustNewClient(c.CartRPC).Conn()),
		Rdb:        rdb,
		Idemp:      idemp,
	}
}

func ctxBackground() context.Context {
	return context.Background()
}
