package blazetrace

import (
	"context"
	"net/http"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const (
	MuxRouteKey = attribute.Key("mux.routes")
)

// ServiceTraceOptions
type ServiceTraceOptions struct {
	tr  trace.Tracer
	tmw func(string) func(next http.Handler) http.Handler
	itf func(ctx context.Context) context.Context
}

// ServiceTraceOption
type ServiceTraceOption func(*ServiceTraceOptions)

// ServiceTracer allows tracing of blaze services
type ServiceTracer interface {
	InjectTracer(ctx context.Context) context.Context
	TracingMiddleware(service string) func(next http.Handler) http.Handler
	StartSpan(ctx context.Context, spanName string, attrs []attribute.KeyValue, opts ...trace.SpanOption) (context.Context, trace.Span)
	EndSpan(span trace.Span)
}

var OtelTracingMiddleware = func(name string) func(next http.Handler) http.Handler {
	return Middleware(name)
}

type serverTracer struct {
	tr  trace.Tracer
	tmw func(string) func(next http.Handler) http.Handler
	itf func(ctx context.Context) context.Context
}

// TracingMiddleware instantiates the tracing middleware
func (s *serverTracer) TracingMiddleware(service string) func(next http.Handler) http.Handler {
	return s.tmw(service)
}
func (s *serverTracer) InjectTracer(ctx context.Context) context.Context {
	return s.itf(ctx)
}

func (s *serverTracer) StartSpan(ctx context.Context, spanName string, attrs []attribute.KeyValue, opts ...trace.SpanOption) (context.Context, trace.Span) {
	// Create a new slice for SpanStartOption
	startOpts := make([]trace.SpanStartOption, 0, len(opts))

	// Iterate through opts and add to startOpts
	for _, opt := range opts {
		if startOpt, ok := opt.(trace.SpanStartOption); ok {
			startOpts = append(startOpts, startOpt)
		}
	}
	startOpts = append(startOpts, trace.WithAttributes(attrs...), trace.WithSpanKind(trace.SpanKindClient))
	return s.tr.Start(ctx, spanName, startOpts...)
}

func (s *serverTracer) EndSpan(span trace.Span) {
	span.End()
}

// NewServiceTracer
func NewServiceTracer(opts ...ServiceTraceOption) ServiceTracer {
	o := &ServiceTraceOptions{
		tr: otel.GetTracerProvider().Tracer("code.cestus.io/blazetrace"),
	}
	for _, opt := range opts {
		opt(o)
	}
	st := &serverTracer{
		tr:  o.tr,
		tmw: o.tmw,
		itf: o.itf,
	}
	// set defaults if needed
	if st.tmw == nil {
		st.tmw = OtelTracingMiddleware
	}
	if st.itf == nil {
		st.itf = func(ctx context.Context) context.Context {
			return WithOtelTracer(ctx, st.tr)
		}
	}
	return st
}

// WithTracer sets a specific tracer to be uses
func WithTracer(tr trace.Tracer) ServiceTraceOption {
	return func(opts *ServiceTraceOptions) {
		opts.tr = tr
	}
}

// WithInjectTracerFunction sets a specific inject tracer function
func WithInjectTracerFunction(itf func(ctx context.Context) context.Context) ServiceTraceOption {
	return func(opts *ServiceTraceOptions) {
		opts.itf = itf
	}
}

// WithTracingMiddleware sets a specific tracing middleware
func WithTracingMiddleware(tmw func(string) func(next http.Handler) http.Handler) ServiceTraceOption {
	return func(opts *ServiceTraceOptions) {
		opts.tmw = tmw
	}
}

// WithAttributes collects attributes into a attribute slice
func WithAttributes(attr ...attribute.KeyValue) []attribute.KeyValue {
	at := append([]attribute.KeyValue{}, attr...)
	return at
}
