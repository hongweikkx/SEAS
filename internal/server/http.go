package server

import (
	v1 "seas/api/seas/v1"
	"seas/internal/conf"
	"seas/internal/service"
	prometheusmetrics "seas/pkg/prometheus"

	"strconv"

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
func NewHTTPServer(c *conf.Server, analysis *service.AnalysisService, examImport *service.ExamImportService, aiAnalysis *AIAnalysisHandler, tp trace.TracerProvider, logger log.Logger) *httptransport.Server {
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
			gorilla.AllowedHeaders([]string{"Accept", "Authorization", "Content-Type", "Last-Event-ID", "Origin"}),
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
	v1.RegisterExamImportHTTPServer(srv, examImport)

	// 覆盖 protobuf 生成的 ImportScores 路由，支持 multipart 文件上传
	r := srv.Route("/")
	r.POST("/seas/api/v1/exams/{exam_id}/scores/import", func(ctx httptransport.Context) error {
		if err := ctx.Request().ParseMultipartForm(32 << 20); err != nil { // 32MB
			return ctx.Result(400, map[string]string{"error": err.Error()})
		}

		file, _, err := ctx.Request().FormFile("file")
		if err != nil {
			return ctx.Result(400, map[string]string{"error": "file required"})
		}
		defer file.Close()

		var vars struct {
			ExamID string `json:"exam_id"`
		}
		if err := ctx.BindVars(&vars); err != nil {
			return ctx.Result(400, map[string]string{"error": err.Error()})
		}
		examID, err := strconv.ParseInt(vars.ExamID, 10, 64)
		if err != nil {
			return ctx.Result(400, map[string]string{"error": "invalid exam_id"})
		}

		reply, err := examImport.ImportScoresFromMultipart(ctx, examID, file)
		if err != nil {
			return ctx.Result(500, map[string]string{"error": err.Error()})
		}

		return ctx.Result(200, reply)
	})

	srv.Handle("/seas/api/v1/ai/analysis", aiAnalysis)
	srv.Handle("/metrics", promhttp.Handler())
	return srv
}
