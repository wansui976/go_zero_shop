package main

import (
	"flag"
	"fmt"

	"github.com/wansui976/go_zero_shop/apps/user/rpc/internal/config"
	"github.com/wansui976/go_zero_shop/apps/user/rpc/internal/server"
	"github.com/wansui976/go_zero_shop/apps/user/rpc/internal/svc"
	"github.com/wansui976/go_zero_shop/apps/user/rpc/user"
	"github.com/wansui976/go_zero_shop/pkg/snowflake"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var configFile = flag.String("f", "etc/user.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)
	ctx := svc.NewServiceContext(c)
	err := snowflake.Init(c.Snowflake.NodeID)
	if err != nil {
		// 初始化失败直接 Panic，因为服务无法生成唯一ID，无法正常工作
		//fmt.Errorf("snowflake init failed: %v", err)
	}

	s := zrpc.MustNewServer(c.RpcServerConf, func(grpcServer *grpc.Server) {
		user.RegisterUserServer(grpcServer, server.NewUserServer(ctx))

		if c.Mode == service.DevMode || c.Mode == service.TestMode {
			reflection.Register(grpcServer)
		}
	})
	defer s.Stop()

	fmt.Printf("Starting rpc server at %s...\n", c.ListenOn)
	s.Start()
}
