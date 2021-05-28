package blazetrace

import "net/http"
import "go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

// The RoundTripperFunc type is an adapter to allow the use of ordinary
// functions as RoundTrippers. If f is a function with the appropriate
// signature, RountTripperFunc(f) is a RoundTripper that calls f.
type RoundTripperFunc func(req *http.Request) (*http.Response, error)

// RoundTrip implements the RoundTripper interface.
func (rt RoundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return rt(r)
}

// OtelClientTrace  adds tracing functionality to an http client (espressed as a roundtripper for api consistency)
func OtelClientTrace(next http.RoundTripper) http.RoundTripper {
	transport := otelhttp.NewTransport(next)
	return transport
}
