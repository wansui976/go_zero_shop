package main

import (
	"flag"
	"fmt"

	"github.com/wansui976/go_zero_shop/apps/app/api/internal/config"
	"github.com/wansui976/go_zero_shop/apps/app/api/internal/handler"
	"github.com/wansui976/go_zero_shop/apps/app/api/internal/svc"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/rest"
)

var configFile = flag.String("f", "etc/api-api.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)

	server := rest.MustNewServer(c.RestConf,
		rest.WithCors("http://localhost:3000"),                            // 允许你的前端域名（文档中的 WithCors 用法）
		rest.WithCorsHeaders("Content-Type", "token", "x-requested-with"), // 文档新增的 WithCorsHeaders（v1.7.1+支持）
	//rest.WithTimeout(3*time.Second), // 可选：统一设置接口超时（文档中的 WithTimeout）
	)
	defer server.Stop()

	ctx := svc.NewServiceContext(c)
	handler.RegisterHandlers(server, ctx)

	fmt.Printf("Starting server at %s:%d...\n", c.Host, c.Port)
	server.Start()
}
