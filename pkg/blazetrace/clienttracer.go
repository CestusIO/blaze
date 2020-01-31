package blazetrace

import (
	"context"

	"net/http"

	"go.opentelemetry.io/otel/api/core"
	"go.opentelemetry.io/otel/api/global"
	"go.opentelemetry.io/otel/api/key"
	"go.opentelemetry.io/otel/api/trace"
	"go.opentelemetry.io/otel/plugin/httptrace"
)

var (
	//ClientName is a trace attribute for the service name
	ClientName = key.New("client.name")
)

//ClientTraceOptions
type ClientTraceOptions struct {
	tr trace.Tracer
}

//ClientTraceOption
type ClientTraceOption func(*ClientTraceOptions)

// ClientTracer allows tracing of blaze services
type ClientTracer interface {
	Inject(ctx context.Context, r *http.Request) (context.Context, *http.Request)
	StartSpan(ctx context.Context, spanName string, attrs []core.KeyValue, opts ...trace.StartOption) (context.Context, trace.Span)
	EndSpan(span trace.Span)
}

type clientTracer struct {
	tr trace.Tracer
	b3 B3
}

func (s *clientTracer) Inject(ctx context.Context, r *http.Request) (context.Context, *http.Request) {
	ctx, req := httptrace.W3C(ctx, r)
	httptrace.Inject(ctx, req)
	s.b3.Inject(ctx, req.Header)
	return ctx, req
}

func (s *clientTracer) StartSpan(ctx context.Context, spanName string, attrs []core.KeyValue, opts ...trace.StartOption) (context.Context, trace.Span) {
	opts = append(opts, trace.WithAttributes(attrs...), trace.WithSpanKind(trace.SpanKindClient))
	return s.tr.Start(ctx, spanName, opts...)
}

func (s *clientTracer) EndSpan(span trace.Span) {
	span.End()
}

// NewClientTracer creates a new tracer
func NewClientTracer(opts ...ClientTraceOption) ClientTracer {
	o := &ClientTraceOptions{
		tr: global.TraceProvider().Tracer("code.cestus.io/blazetrace"),
	}
	for _, opt := range opts {
		opt(o)
	}
	st := &clientTracer{
		tr: o.tr,
	}
	return st
}

// WithClientTracer sets a specific tracer to be uses
func WithClientTracer(tr trace.Tracer) ClientTraceOption {
	return func(opts *ClientTraceOptions) {
		opts.tr = tr
	}
}
