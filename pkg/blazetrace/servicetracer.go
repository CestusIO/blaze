package blazetrace

import (
	"context"
	"net/http"

	"go.opentelemetry.io/otel/api/key"
	"go.opentelemetry.io/otel/api/trace"

	"go.opentelemetry.io/otel/api/core"
	"go.opentelemetry.io/otel/api/distributedcontext"
	"go.opentelemetry.io/otel/api/global"
	"go.opentelemetry.io/otel/plugin/httptrace"
)

var (
	//ServiceName is a trace attribute for the service name
	ServiceName = key.New("service.name")
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
	StartSpan(ctx context.Context, spanName string, psd ParentSpanDescriptor, attrs []core.KeyValue, opts ...trace.StartOption) (context.Context, trace.Span)
	EndSpan(span trace.Span)
}

type serverTracer struct {
	tr trace.Tracer
}

// ParentSpanDescriptor dexcribes the parent span
type ParentSpanDescriptor struct {
	spanContext core.SpanContext
	attrs       []core.KeyValue
	entries     []core.KeyValue
}

func (s *serverTracer) Extract(req *http.Request) (context.Context, *http.Request, ParentSpanDescriptor) {
	attrs, entries, sctx := httptrace.Extract(req.Context(), req)
	psd := ParentSpanDescriptor{
		spanContext: sctx,
		attrs:       attrs,
		entries:     entries,
	}
	req = req.WithContext(distributedcontext.NewContext(req.Context()))
	return req.Context(), req, psd
}

func (s *serverTracer) StartSpan(ctx context.Context, spanName string, psd ParentSpanDescriptor, attrs []core.KeyValue, opts ...trace.StartOption) (context.Context, trace.Span) {
	attrs = append(attrs, psd.attrs...)
	opts = append(opts, trace.WithAttributes(attrs...), trace.WithSpanKind(trace.SpanKindServer), trace.ChildOf(psd.spanContext))
	return s.tr.Start(ctx, spanName, opts...)
}

func (s *serverTracer) EndSpan(span trace.Span) {
	span.End()
}

// NewServiceTracer
func NewServiceTracer(opts ...ServiceTraceOption) ServiceTracer {
	o := &ServiceTraceOptions{
		tr: global.TraceProvider().Tracer("code.cestus.io/blazetrace"),
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
func WithAttributes(attr ...core.KeyValue) []core.KeyValue {
	at := append([]core.KeyValue{}, attr...)
	return at
}
