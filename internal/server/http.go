package server

import (
	v1 "seas/api/seas/v1"
	"seas/internal/conf"
	"seas/internal/service"
	prometheusmetrics "seas/pkg/prometheus"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/logging"
	"github.com/go-kratos/kratos/v2/middleware/metrics"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/middleware/tracing"
	"github.com/go-kratos/kratos/v2/middleware/validate"
	httptransport "github.com/go-kratos/kratos/v2/transport/http"
	gorilla "github.com/gorilla/handlers"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel/trace"
)

// NewHTTPServer new an HTTP server.
func NewHTTPServer(c *conf.Server, analysis *service.AnalysisService, mcpServer *MCPServer, chatHandler *ChatHandler, tp trace.TracerProvider, logger log.Logger) *httptransport.Server {
	var opts = []httptransport.ServerOption{
		httptransport.Middleware(
			recovery.Recovery(),
			tracing.Server(tracing.WithTracerProvider(tp)),
			logging.Server(logger),
			validate.Validator(),
			metrics.Server(
				metrics.WithSeconds(prometheusmetrics.MetricSeconds),
				metrics.WithRequests(prometheusmetrics.MetricRequests),
			),
		),
		httptransport.Filter(gorilla.CORS(
			gorilla.AllowedOrigins([]string{"*"}),
			gorilla.AllowedMethods([]string{"GET", "POST", "DELETE", "OPTIONS"}),
			gorilla.AllowedHeaders([]string{"Accept", "Authorization", "Content-Type", "Last-Event-ID", "Mcp-Protocol-Version", "Mcp-Session-Id", "Origin"}),
			gorilla.ExposedHeaders([]string{"Mcp-Session-Id"}),
		)),
	}
	if c.Http.Network != "" {
		opts = append(opts, httptransport.Network(c.Http.Network))
	}
	if c.Http.Addr != "" {
		opts = append(opts, httptransport.Address(c.Http.Addr))
	}
	if c.Http.Timeout != nil {
		opts = append(opts, httptransport.Timeout(c.Http.Timeout.AsDuration()))
	}
	srv := httptransport.NewServer(opts...)
	v1.RegisterAnalysisHTTPServer(srv, analysis)
	srv.Handle("/chat", chatHandler)
	srv.Handle("/mcp", mcpServer.Handler())
	srv.Handle("/metrics", promhttp.Handler())
	return srv
}
