// Package main
package main

import (
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"

	"seas/pkg/jwt"
	"seas/pkg/zaplog"

	"seas/internal/conf"

	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/config/file"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/tracing"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/go-kratos/kratos/v2/transport/http"
	_ "go.uber.org/automaxprocs"
)

// go build -ldflags "-X main.Version=x.y.z"
var (
	// Name is the name of the compiled software.
	Name = "seas"
	// Version is the version of the compiled software.
	Version = "0.0.1"
	// flagConf is the config flag.
	flagConf string
	id, _    = os.Hostname()
)

func newApp(logger log.Logger, gs *grpc.Server, hs *http.Server) *kratos.App {
	return kratos.New(
		kratos.ID(id),
		kratos.Name(Name),
		kratos.Version(Version),
		kratos.Metadata(map[string]string{}),
		kratos.Logger(logger),
		kratos.Server(
			gs,
			hs,
		),
	)
}

func NewTraceProvider() trace.TracerProvider {
	exporter, err := stdouttrace.New(stdouttrace.WithWriter(io.Discard))
	if err != nil {
		panic(err)
	}
	tp := tracesdk.NewTracerProvider(
		tracesdk.WithBatcher(exporter),
		tracesdk.WithResource(resource.NewSchemaless()))
	return tp
}

func main() {
	// log
	zapLogger := zaplog.InitLogger()
	defer func(zapLogger *zaplog.Logger) {
		_ = zapLogger.Close()
	}(zapLogger)
	logger := log.With(zapLogger,
		"ts", log.DefaultTimestamp,
		"caller", log.DefaultCaller,
		"service.id", id,
		"service.name", Name,
		"service.version", Version,
		"trace.id", tracing.TraceID(),
		"span.id", tracing.SpanID(),
	)
	log.SetLogger(logger)

	// config
	flag.StringVar(&flagConf, "conf", "../../configs/config.yaml", "config path, eg: -conf config.yaml")
	flag.Parse()

	c := config.New(
		config.WithSource(

			file.NewSource(flagConf),
		),
	)
	defer func(c config.Config) {
		err := c.Close()
		if err != nil {
			log.Errorf("close config error, %s", err.Error())
		}
	}(c)
	if err := c.Load(); err != nil {
		panic(err)
	}
	var bc conf.Bootstrap
	if err := c.Scan(&bc); err != nil {
		panic(err)
	}

	// 将 SQLite 相对路径转为绝对路径，避免 kratos run 时 CWD 变化导致找不到数据库
	if bc.Data != nil && bc.Data.Database != nil {
		source := bc.Data.Database.Source
		if strings.HasPrefix(source, "file:") && strings.Contains(source, "./") {
			absConf, err := filepath.Abs(flagConf)
			if err == nil {
				rootDir := filepath.Dir(filepath.Dir(absConf)) // configs/config.yaml → 项目根目录
				// 提取 file: 后的路径部分（去掉查询参数）
				pathPart := strings.TrimPrefix(source, "file:")
				qIdx := strings.Index(pathPart, "?")
				var query string
				if qIdx >= 0 {
					query = pathPart[qIdx:]
					pathPart = pathPart[:qIdx]
				}
				if filepath.IsAbs(pathPart) {
					// 已经是绝对路径，无需处理
				} else {
					absPath := filepath.Join(rootDir, pathPart)
					bc.Data.Database.Source = "file:" + absPath + query
				}
			}
		}
	}

	// 初始化 JWT 密钥
	jwt.Init(bc.Auth.GetJwtSecret())

	log.Infof("server env=%s, dev_mode=%v", bc.Env, bc.Env == conf.EnvDev)

	app, cleanup, err := wireApp(bc.Server, bc.Data, bc.Llm, bc.Auth, bc.Env, logger)
	if err != nil {
		panic(err)
	}
	defer cleanup()

	// start and wait for stop signal
	if err = app.Run(); err != nil {
		panic(err)
	}
	log.Info("seas stop")
}
