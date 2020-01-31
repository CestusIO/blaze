package blazetrace

import (
	"context"
	"fmt"
	"strings"

	"go.opentelemetry.io/otel/api/core"
	"go.opentelemetry.io/otel/api/distributedcontext"
	"go.opentelemetry.io/otel/api/trace"
)

// TextFormat is an interface that specifies methods to inject and extract SpanContext
// and distributed context into/from a carrier using Supplier interface.
// For example, HTTP Trace Context propagator would encode SpanContext into W3C Trace
// Context Header and set the header into HttpRequest.
type TextFormat interface {
	// Inject method retrieves current SpanContext from the ctx, encodes it into propagator
	// specific format and then injects the encoded SpanContext using supplier into a carrier
	// associated with the supplier. It also takes a correlationCtx whose values will be
	// injected into a carrier using the supplier.
	Inject(ctx context.Context, supplier Supplier)

	// Extract method retrieves encoded SpanContext using supplier from the associated carrier.
	// It decodes the SpanContext and returns it and a baggage of correlated context.
	// If no SpanContext was retrieved OR if the retrieved SpanContext is invalid then
	// an empty SpanContext is returned.
	Extract(ctx context.Context, supplier Supplier) (core.SpanContext, distributedcontext.Map)

	// GetAllKeys returns all the keys that this propagator injects/extracts into/from a
	// carrier. The use cases for this are
	// * allow pre-allocation of fields, especially in systems like gRPC Metadata
	// * allow a single-pass over an iterator (ex OpenTracing has no getter in TextMap)
	GetAllKeys() []string
}

// Supplier is an interface that specifies methods to retrieve and store
// value for a key to an associated carrier.
// Get method retrieves the value for a given key.
// Set method stores the value for a given key.
type Supplier interface {
	Get(key string) string
	Set(key string, value string)
}

const (
	B3SingleHeader       = "X-B3"
	B3DebugFlagHeader    = "X-B3-Flags"
	B3TraceIDHeader      = "X-B3-TraceId"
	B3SpanIDHeader       = "X-B3-SpanId"
	B3SampledHeader      = "X-B3-Sampled"
	B3ParentSpanIDHeader = "X-B3-ParentSpanId"
)

// B3 propagator serializes core.SpanContext to/from B3 Headers.
// This propagator supports both version of B3 headers,
//  1. Single Header :
//    X-B3: {TraceId}-{SpanId}-{SamplingState}-{ParentSpanId}
//  2. Multiple Headers:
//    X-B3-TraceId: {TraceId}
//    X-B3-ParentSpanId: {ParentSpanId}
//    X-B3-SpanId: {SpanId}
//    X-B3-Sampled: {SamplingState}
//    X-B3-Flags: {DebugFlag}
//
// If SingleHeader is set to true then X-B3 header is used to inject and extract. Otherwise,
// separate headers are used to inject and extract.
type B3 struct {
	SingleHeader bool
}

var _ TextFormat = B3{}

func (b3 B3) Inject(ctx context.Context, supplier Supplier) {
	sc := trace.SpanFromContext(ctx).SpanContext()
	if sc.IsValid() {
		if b3.SingleHeader {
			sampled := sc.TraceFlags & core.TraceFlagsSampled
			supplier.Set(B3SingleHeader,
				fmt.Sprintf("%s-%.16x-%.1d", sc.TraceIDString(), sc.SpanID, sampled))
		} else {
			supplier.Set(B3TraceIDHeader, sc.TraceIDString())
			supplier.Set(B3SpanIDHeader,
				fmt.Sprintf("%.16x", sc.SpanID))

			var sampled string
			if sc.IsSampled() {
				sampled = "1"
			} else {
				sampled = "0"
			}
			supplier.Set(B3SampledHeader, sampled)
		}
	}
}

// Extract retrieves B3 Headers from the supplier
func (b3 B3) Extract(ctx context.Context, supplier Supplier) (core.SpanContext, distributedcontext.Map) {
	if b3.SingleHeader {
		return b3.extractSingleHeader(supplier), distributedcontext.NewEmptyMap()
	}
	return b3.extract(supplier), distributedcontext.NewEmptyMap()
}

func (b3 B3) GetAllKeys() []string {
	if b3.SingleHeader {
		return []string{B3SingleHeader}
	}
	return []string{B3TraceIDHeader, B3SpanIDHeader, B3SampledHeader}
}

func (b3 B3) extract(supplier Supplier) core.SpanContext {
	tid, err := core.TraceIDFromHex(supplier.Get(B3TraceIDHeader))
	if err != nil {
		return core.EmptySpanContext()
	}
	sid, err := core.SpanIDFromHex(supplier.Get(B3SpanIDHeader))
	if err != nil {
		return core.EmptySpanContext()
	}
	sampled, ok := b3.extractSampledState(supplier.Get(B3SampledHeader))
	if !ok {
		return core.EmptySpanContext()
	}

	debug, ok := b3.extracDebugFlag(supplier.Get(B3DebugFlagHeader))
	if !ok {
		return core.EmptySpanContext()
	}
	if debug == core.TraceFlagsSampled {
		sampled = core.TraceFlagsSampled
	}

	sc := core.SpanContext{
		TraceID:    tid,
		SpanID:     sid,
		TraceFlags: sampled,
	}

	if !sc.IsValid() {
		return core.EmptySpanContext()
	}

	return sc
}

func (b3 B3) extractSingleHeader(supplier Supplier) core.SpanContext {
	h := supplier.Get(B3SingleHeader)
	if h == "" || h == "0" {
		core.EmptySpanContext()
	}
	sc := core.SpanContext{}
	parts := strings.Split(h, "-")
	l := len(parts)
	if l > 4 {
		return core.EmptySpanContext()
	}

	if l < 2 {
		return core.EmptySpanContext()
	}

	var err error
	sc.TraceID, err = core.TraceIDFromHex(parts[0])
	if err != nil {
		return core.EmptySpanContext()
	}

	sc.SpanID, err = core.SpanIDFromHex(parts[1])
	if err != nil {
		return core.EmptySpanContext()
	}

	if l > 2 {
		var ok bool
		sc.TraceFlags, ok = b3.extractSampledState(parts[2])
		if !ok {
			return core.EmptySpanContext()
		}
	}
	if l == 4 {
		_, err = core.SpanIDFromHex(parts[3])
		if err != nil {
			return core.EmptySpanContext()
		}
	}

	if !sc.IsValid() {
		return core.EmptySpanContext()
	}

	return sc
}

// extractSampledState parses the value of the X-B3-Sampled b3Header.
func (b3 B3) extractSampledState(sampled string) (flag byte, ok bool) {
	switch sampled {
	case "", "0":
		return 0, true
	case "1":
		return core.TraceFlagsSampled, true
	case "true":
		if !b3.SingleHeader {
			return core.TraceFlagsSampled, true
		}
	case "d":
		if b3.SingleHeader {
			return core.TraceFlagsSampled, true
		}
	}
	return 0, false
}

// extracDebugFlag parses the value of the X-B3-Sampled b3Header.
func (b3 B3) extracDebugFlag(debug string) (flag byte, ok bool) {
	switch debug {
	case "", "0":
		return 0, true
	case "1":
		return core.TraceFlagsSampled, true
	}
	return 0, false
}
