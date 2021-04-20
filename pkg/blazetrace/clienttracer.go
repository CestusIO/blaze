package blazetrace

import (
	"context"

	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/httptrace/otelhttptrace"
	"go.opentelemetry.io/contrib/propagators/b3"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

var (
	//ClientName is a trace attribute for the service name
	ClientName = attribute.Key("client.name")
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
	StartSpan(ctx context.Context, spanName string, attrs []attribute.KeyValue, opts ...trace.SpanOption) (context.Context, trace.Span)
	EndSpan(span trace.Span)
}

type clientTracer struct {
	tr trace.Tracer
	b3 b3.B3
}

func (s *clientTracer) Inject(ctx context.Context, r *http.Request) (context.Context, *http.Request) {
	ctx, req := otelhttptrace.W3C(ctx, r)
	otelhttptrace.Inject(ctx, req)
	carrier := propagation.HeaderCarrier(req.Header)
	s.b3.Inject(ctx, carrier)
	return ctx, req
}

func (s *clientTracer) StartSpan(ctx context.Context, spanName string, attrs []attribute.KeyValue, opts ...trace.SpanOption) (context.Context, trace.Span) {
	opts = append(opts, trace.WithAttributes(attrs...), trace.WithSpanKind(trace.SpanKindClient))
	return s.tr.Start(ctx, spanName, opts...)
}

func (s *clientTracer) EndSpan(span trace.Span) {
	span.End()
}

// NewClientTracer creates a new tracer
func NewClientTracer(opts ...ClientTraceOption) ClientTracer {
	o := &ClientTraceOptions{
		tr: otel.GetTracerProvider().Tracer("code.cestus.io/blazetrace"),
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
