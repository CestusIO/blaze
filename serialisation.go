package blaze

import (
	"errors"
	"strconv"
)

// ErrorJSON is JSON serialization for blaze errors
type ErrorJSON struct {
	Code string            `json:"code"`
	Msg  string            `json:"msg"`
	Type string            `json:"blaze_type"`
	Meta map[string]string `json:"meta,omitempty"`
}

// ErrorToErrorJSON concerts a Error into a ErrorJSON struct
func ErrorToErrorJSON(e Error) (ErrorJSON, error) {
	// make sure that msg is not too large
	msg := e.Msg()
	if len(msg) > 1e6 {
		msg = msg[:1e6]
	}

	be := ErrorJSON{
		Code: strconv.Itoa(ServerHTTPStatusFromErrorType(e)),
		Msg:  msg,
		Type: e.Type(),
		Meta: e.MetaMap(),
	}
	var err error
	err = e
	switch e.(type) {
	case Error:
		{
			err = errors.Unwrap(e)

		}
	}
	switch err.(type) {
	case *InternalErrorType:
		ie := err.(*InternalErrorType)
		if ie.err != nil {
			be.Meta["wrappedInternalError"] = err.Error()
		}
	}
	return be, nil
}

// ErrorJSONToError converts a ErrorJSON struct into an Error
func ErrorJSONToError(j ErrorJSON) (Error, error) {
	// make sure that msg is not too large
	if len(j.Msg) > 1e6 {
		j.Msg = j.Msg[:1e6]
	}
	e, err := registry.Contruct(j.Type, j.Msg)
	if err != nil {
		return nil, err
	}
	// Special case for InternalErrorWith
	if es, ok := j.Meta["wrappedInternalError"]; ok {
		if j.Type == "*blaze.InternalErrorType" {
			e = NewError(&InternalErrorType{err: errors.New(es)}, j.Msg)
		}
	}
	// end special case

	for k, v := range j.Meta {
		if k == "wrappedInternalError" {
			continue
		}
		e = e.WithMeta(k, v)
	}
	return e, nil
}

type errorCreateFunc func(msg string) Error
type errorRegistry map[string]errorCreateFunc

var registry errorRegistry = map[string]errorCreateFunc{
	"*blaze.CanceledErrorType":           func(msg string) Error { return NewError(&CanceledErrorType{}, msg) },
	"*blaze.MalformedErrorType":          func(msg string) Error { return NewError(&MalformedErrorType{}, msg) },
	"*blaze.DeadlineExceededErrorType":   func(msg string) Error { return NewError(&DeadlineExceededErrorType{}, msg) },
	"*blaze.NotFoundErrorType":           func(msg string) Error { return NewError(&NotFoundErrorType{}, msg) },
	"*blaze.BadRouteErrorType":           func(msg string) Error { return NewError(&BadRouteErrorType{}, msg) },
	"*blaze.InvalidArgumentErrorType":    func(msg string) Error { return NewError(&InvalidArgumentErrorType{}, msg) },
	"*blaze.AlreadyExistsErrorType":      func(msg string) Error { return NewError(&AlreadyExistsErrorType{}, msg) },
	"*blaze.PermissionDeniedErrorType":   func(msg string) Error { return NewError(&PermissionDeniedErrorType{}, msg) },
	"*blaze.UnauthenticatedErrorType":    func(msg string) Error { return NewError(&UnauthenticatedErrorType{}, msg) },
	"*blaze.ResourceExhaustedErrorType":  func(msg string) Error { return NewError(&ResourceExhaustedErrorType{}, msg) },
	"*blaze.FailedPreconditionErrorType": func(msg string) Error { return NewError(&FailedPreconditionErrorType{}, msg) },
	"*blaze.AbortedErrorType":            func(msg string) Error { return NewError(&AbortedErrorType{}, msg) },
	"*blaze.OutOfRangeErrorType":         func(msg string) Error { return NewError(&OutOfRangeErrorType{}, msg) },
	"*blaze.UnimplementedErrorType":      func(msg string) Error { return NewError(&UnimplementedErrorType{}, msg) },
	"*blaze.InternalErrorType":           func(msg string) Error { return NewError(&InternalErrorType{}, msg) },
	"*blaze.UnavailableErrorType":        func(msg string) Error { return NewError(&UnavailableErrorType{}, msg) },
	"*blaze.DataLossErrorType":           func(msg string) Error { return NewError(&DataLossErrorType{}, msg) },
}

func (r errorRegistry) Contruct(name string, msg string) (Error, error) {
	if cf, ok := r[name]; ok {
		e := cf(msg)
		return e, nil
	}
	return nil, errors.New("Not Registered")
}
