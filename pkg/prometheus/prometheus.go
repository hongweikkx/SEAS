package prometheus

import (
	"github.com/go-kratos/kratos/v2/middleware/metrics"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

var (
	MetricRequests metric.Int64Counter
	MetricSeconds  metric.Float64Histogram
)

func Init() {
	exporter, err := prometheus.New()
	if err != nil {
		panic(err)
	}
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(exporter))
	meter := provider.Meter("metric")

	MetricRequests, err = metrics.DefaultRequestsCounter(meter, metrics.DefaultServerRequestsCounterName)
	if err != nil {
		panic(err)
	}

	MetricSeconds, err = metrics.DefaultSecondsHistogram(meter, metrics.DefaultServerSecondsHistogramName)
	if err != nil {
		panic(err)
	}
}
