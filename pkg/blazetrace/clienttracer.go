package blazetrace

import (
	"context"

	"net/http/httptrace"

	"go.opentelemetry.io/contrib/instrumentation/net/http/httptrace/otelhttptrace"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
)

var (
	//ClientName is a trace attribute for the service name
	ClientName = semconv.PeerServiceKey
)

// ClientTraceOptions
type ClientTraceOptions struct {
	tr trace.Tracer
}

// ClientTraceOption
type ClientTraceOption func(*ClientTraceOptions)

// ClientTracer allows tracing of blaze services
type ClientTracer interface {
	StartSpan(ctx context.Context, spanName string, attrs []attribute.KeyValue, opts ...trace.SpanOption) (context.Context, trace.Span)
	EndSpan(span trace.Span)
	AnnotateWithClientTrace(ctx context.Context) context.Context
}

type clientTracer struct {
	tr trace.Tracer
}

func (s *clientTracer) StartSpan(ctx context.Context, spanName string, attrs []attribute.KeyValue, opts ...trace.SpanOption) (context.Context, trace.Span) {
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

func (s *clientTracer) EndSpan(span trace.Span) {
	span.End()
}

func (s *clientTracer) AnnotateWithClientTrace(ctx context.Context) context.Context {
	return httptrace.WithClientTrace(ctx, otelhttptrace.NewClientTrace(ctx))
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
