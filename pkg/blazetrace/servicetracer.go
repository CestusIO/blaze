package blazetrace

import (
	"context"
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/httptrace/otelhttptrace"
	"go.opentelemetry.io/contrib/propagators/b3"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

var (
	//ServiceName is a trace attribute for the service name
	ServiceName = attribute.Key("service.name")
	//EmptyParentSpanDescriptor is a descritor for an empty span
	EmptyParentSpanDescriptor = ParentSpanDescriptor{}
)

//ServiceTraceOptions
type ServiceTraceOptions struct {
	tr trace.Tracer
}

//ServiceTraceOption
type ServiceTraceOption func(*ServiceTraceOptions)

// ServiceTracer allows tracing of blaze services
type ServiceTracer interface {
	Extract(r *http.Request) (context.Context, *http.Request, ParentSpanDescriptor)
	StartSpan(ctx context.Context, spanName string, psd ParentSpanDescriptor, attrs []attribute.KeyValue, opts ...trace.SpanOption) (context.Context, trace.Span)
	EndSpan(span trace.Span)
}

type serverTracer struct {
	tr trace.Tracer
	b3 b3.B3
}

// ParentSpanDescriptor dexcribes the parent span
type ParentSpanDescriptor struct {
	spanContext trace.SpanContext
	attrs       []attribute.KeyValue
	entries     []attribute.KeyValue
}

func (s *serverTracer) Extract(req *http.Request) (context.Context, *http.Request, ParentSpanDescriptor) {
	attrs, entries, sctx := otelhttptrace.Extract(req.Context(), req)
	psd := ParentSpanDescriptor{
		spanContext: sctx,
		attrs:       attrs,
		entries:     entries,
	}
	req = req.WithContext(baggage.ContextWithValues(req.Context()))

	if !psd.spanContext.IsValid() {
		carrier := propagation.HeaderCarrier(req.Header)
		spContext := s.b3.Extract(req.Context(), carrier)
		psd.spanContext = trace.SpanContextFromContext(spContext)
	}
	return req.Context(), req, psd
}

func (s *serverTracer) StartSpan(ctx context.Context, spanName string, psd ParentSpanDescriptor, attrs []attribute.KeyValue, opts ...trace.SpanOption) (context.Context, trace.Span) {
	attrs = append(attrs, psd.attrs...)
	opts = append(opts, trace.WithAttributes(attrs...), trace.WithSpanKind(trace.SpanKindServer))
	return s.tr.Start(ctx, spanName, opts...)
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
		tr: o.tr,
	}
	return st
}

// WithTracer sets a specific tracer to be uses
func WithTracer(tr trace.Tracer) ServiceTraceOption {
	return func(opts *ServiceTraceOptions) {
		opts.tr = tr
	}
}

//WithAttributes collects attributes into a attribute slice
func WithAttributes(attr ...attribute.KeyValue) []attribute.KeyValue {
	at := append([]attribute.KeyValue{}, attr...)
	return at
}
