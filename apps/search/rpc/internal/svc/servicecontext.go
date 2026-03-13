package svc

import (
	"github.com/elastic/go-elasticsearch/v9"
	"github.com/wansui976/go_zero_shop/apps/product/rpc/product"
	"github.com/wansui976/go_zero_shop/apps/product/rpc/productclient"
	"github.com/wansui976/go_zero_shop/apps/search/rpc/internal/config"
	"github.com/wansui976/go_zero_shop/pkg/es"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/zrpc"
)

const IndexName = "product_index"

type ServiceContext struct {
	Config     config.Config
	EsClient   *elasticsearch.Client
	ProductRpc product.ProductClient
	Cache      *redis.Redis // 搜索结果缓存
}

func NewServiceContext(c config.Config) *ServiceContext {
	return &ServiceContext{
		Config:     c,
		EsClient:   es.GetESClient(c.Es.Host, c.Es.Username, c.Es.Password),
		ProductRpc: productclient.NewProduct(zrpc.MustNewClient(c.ProductRpc)),
		Cache:      redis.MustNewRedis(c.CacheRedis),
	}
}
