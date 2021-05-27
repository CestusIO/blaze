package blazetrace

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

type tracer struct{}

var tracerKey = &tracer{}

// WithOtelTracer adds the tracer to the context
func WithOtelTracer(ctx context.Context, tracer trace.Tracer) context.Context {
	return context.WithValue(ctx, tracerKey, tracer)
}

func GetOtelTracer(ctx context.Context) trace.Tracer {
	if tracer, ok := ctx.Value(tracerKey).(trace.Tracer); ok {
		return tracer
	}
	return otel.GetTracerProvider().Tracer("")
}
