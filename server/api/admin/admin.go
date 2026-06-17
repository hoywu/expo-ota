// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package main

import (
	"context"
	"flag"
	"fmt"

	"github.com/hoywu/expo-ota/server/api/admin/internal/config"
	"github.com/hoywu/expo-ota/server/api/admin/internal/handler"
	"github.com/hoywu/expo-ota/server/api/admin/internal/httperr"
	adminlogic "github.com/hoywu/expo-ota/server/api/admin/internal/logic/admin"
	"github.com/hoywu/expo-ota/server/api/admin/internal/middleware"
	"github.com/hoywu/expo-ota/server/api/admin/internal/reqmeta"
	"github.com/hoywu/expo-ota/server/api/admin/internal/svc"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"
)

var configFile = flag.String("f", "etc/admin-api.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c, conf.UseEnv())
	httpx.SetErrorHandlerCtx(httperr.ToErrorResponse)

	server := rest.MustNewServer(c.RestConf)
	defer server.Stop()

	ctx := svc.NewServiceContext(c)

	if err := adminlogic.BootstrapInitialAdmin(context.Background(), ctx); err != nil {
		logx.Errorf("bootstrap initial admin failed: %v", err)
	}

	server.Use(reqmeta.Middleware)
	server.Use(middleware.NewApiTokenAuthMiddleware(ctx).Handle)
	handler.RegisterHandlers(server, ctx)

	fmt.Printf("Starting server at %s:%d...\n", c.Host, c.Port)
	server.Start()
}
