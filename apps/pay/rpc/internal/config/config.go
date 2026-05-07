package config

import (
	"github.com/zeromicro/go-zero/core/stores/cache"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	zrpc.RpcServerConf
	DataSource  string
	CacheRedis  cache.CacheConf
	OrderRpc    zrpc.RpcClientConf
	Snowflake   struct {
		NodeID int64 `json:"NodeID"`
	} `json:"Snowflake"`
	BizRedis         redis.RedisConf
	PayWebhookSecret string `json:"PayWebhookSecret"`

	// 支付配置
	WechatPay struct {
		AppID       string `json:"appId"`
		MchID       string `json:"mchId"`
		ApiKey      string `json:"apiKey"`
		NotifyURL   string `json:"notifyURL"`
		TradeType   string `json:"tradeType"` // JSAPI, NATIVE, APP
	} `json:"WechatPay"`

	Alipay struct {
		AppID       string `json:"appId"`
		PrivateKey  string `json:"privateKey"`
		PublicKey   string `json:"publicKey"`
		NotifyURL   string `json:"notifyURL"`
		ReturnURL   string `json:"returnURL"`
	} `json:"Alipay`
}
