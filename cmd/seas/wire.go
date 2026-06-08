//go:build wireinject
// +build wireinject

// The build tag makes sure the stub is not built in the final build.

package main

import (
	"seas/internal/biz"
	"seas/internal/conf"
	"seas/internal/data"
	"seas/internal/server"
	"seas/internal/service"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"
)

// wireApp init kratos application.
func wireApp(*conf.Server, *conf.Data, *conf.LLM, *conf.Auth, string, log.Logger) (*kratos.App, func(), error) {
	panic(wire.Build(server.ProviderSet, data.ProviderSet, biz.ProviderSet, service.ProviderSet, NewTraceProvider, newApp))
}

// 注：当前已禁用 gRPC 服务器，仅保留 HTTP 服务器。
// 如需恢复 gRPC，将 newApp 签名改回 (logger, gs, hs) 并在 kratos.Server() 中加入 gs 即可。
