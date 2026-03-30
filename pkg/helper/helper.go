package helper

import (
	"context"
	"crypto/rand"

	"go.opentelemetry.io/otel/trace"
)

// DetachedTraceContext 返回独立context，保留原trace，如果没有则生成新的 trace
//   - 如果 ctx 中有 trace 信息，则提取并注入新的 context.Background() 中；
//   - 如果没有，则生成新的 trace 信息。
func DetachedTraceContext(ctx context.Context) context.Context {
	spanCtx := trace.SpanContextFromContext(ctx)
	if !spanCtx.IsValid() {
		// 没有 trace 信息，构造新的 trace/span ID
		var tid [16]byte
		var sid [8]byte
		_, _ = rand.Read(tid[:])
		_, _ = rand.Read(sid[:])
		spanCtx = trace.NewSpanContext(trace.SpanContextConfig{
			TraceID:    tid,
			SpanID:     sid,
			TraceFlags: trace.FlagsSampled,
			Remote:     false,
		})
	}
	// 构造一个新的 context，避免被原 ctx 的取消影响
	return trace.ContextWithSpanContext(context.Background(), spanCtx)
}

// WithTraceContext 保留原context，如果没有trace，则生成新的 trace
func WithTraceContext(ctx context.Context) context.Context {
	spanCtx := trace.SpanContextFromContext(ctx)
	// 如果已经有有效 trace，则直接返回原 ctx
	if spanCtx.IsValid() {
		return ctx
	}
	// 没有 trace，生成新的 traceID 和 spanID
	var tid [16]byte
	var sid [8]byte
	_, _ = rand.Read(tid[:])
	_, _ = rand.Read(sid[:])
	newSpanCtx := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    tid,
		SpanID:     sid,
		TraceFlags: trace.FlagsSampled,
		Remote:     false,
	})
	// 将新 span 注入到原 ctx 中
	return trace.ContextWithSpanContext(ctx, newSpanCtx)

}
