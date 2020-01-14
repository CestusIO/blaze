/*
  File: \error.go
  Created Date: Monday, November 4th 2019, 7:16:09 pm
  Author: Ralf Mueller
  -----
  Last Modified:
  Modified By:
  -----
  Copyright (c) 2019 Ralf Mueller


  Licensed under the Apache License, Version 2.0 (the "License");
  you may not use this file except in compliance with the License.
  You may obtain a copy of the License at

   http: //www.apache.org/licenses/LICENSE-2.0

  Unless required by applicable law or agreed to in writing, software
  distributed under the License is distributed on an 'AS IS' BASIS,
  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
  See the License for the specific language governing permissions and
  limitations under the License.
  -----
  HISTORY:
  Date      	By	Comments
  ----------	---	----------------------------------------------------------
*/

package blaze

import (
	"errors"
	"fmt"
)

// Error represents an error in a Blaze service call.
type Error interface {
	// Type returns the type of the error
	Type() string
	// Msg returns a human-readable, unstructured messages describing the error.
	Msg() string

	// WithMeta returns a copy of the Error with the given key-value pair attached
	// as metadata. If the key is already set, it is overwritten.
	WithMeta(key string, val string) Error

	// Meta returns the stored value for the given key. If the key has no set
	// value, Meta returns an empty string. There is no way to distinguish between
	// an unset value and an explicit empty string.
	Meta(key string) string

	// MetaMap returns a copy of the complete key-value metadata map stored on the error.
	MetaMap() map[string]string

	// Error returns a string of the form "blaze error <Type>: <Msg>"
	Error() string

	//Unwrap returns the wrapped error
	Unwrap() error
}

// blaze.Error implementation
type blerr struct {
	err  error
	msg  string
	meta map[string]string
}

func (e *blerr) Type() string { return fmt.Sprintf("%T", e.err) }
func (e *blerr) Msg() string  { return e.msg }
func (e *blerr) Meta(key string) string {
	if e.meta != nil {
		return e.meta[key] // also returns "" if key is not in meta map
	}
	return ""
}

func (e *blerr) WithMeta(key string, value string) Error {
	newErr := &blerr{
		err:  e.err,
		msg:  e.msg,
		meta: make(map[string]string, len(e.meta)),
	}
	for k, v := range e.meta {
		newErr.meta[k] = v
	}
	newErr.meta[key] = value
	return newErr
}

func (e *blerr) MetaMap() map[string]string {
	meta := make(map[string]string, len(e.meta))
	for k, v := range e.meta {
		meta[k] = v
	}
	return meta
}

func (e *blerr) Error() string {
	return fmt.Sprintf("blaze error %s: %s", e.Type(), e.msg)
}

func (e *blerr) Unwrap() error { return e.err }

func (e *blerr) Is(err error) bool {
	var er interface{} = err
	assuredErr, ok := er.(Error)
	if ok {
		return e.Unwrap().Error() == assuredErr.Unwrap().Error()
	}
	return false
}

// NewError is the generic constructor for a blaze.Error. The error must be
// one of the valid predefined ones in errors.go, otherwise it will be converted to an
// error {type: Internal, msg: "invalid error type {{code}}"}. If you need to
// add metadata, use .WithMeta(key, value) method after building the error.
func NewError(err error, msg string) Error {
	if IsError(err) {
		return &blerr{
			err: err,
			msg: msg,
		}
	}
	return &blerr{
		err: &InternalErrorType{
			err: err,
		},
		msg: func(err error, msg string) string {
			if err != nil {
				return fmt.Sprintf("Internal error: %s: %s", msg, err.Error())
			}
			return msg
		}(err, msg),
	}
}

// IsError checks if the passed error is a blaze error
func IsError(err error) bool {
	return ServerHTTPStatusFromErrorType(err) != 0
}

// ServerHTTPStatusFromErrorType maps a blaze error type into a similar HTTP
// response status. It is used by the blaze server handler to set the HTTP
// response status code. Returns 0 if the error is not a blaze error.
func ServerHTTPStatusFromErrorType(err error) int {
	switch err.(type) {
	case Error:
		{
			err = errors.Unwrap(err)
		}
	}
	switch err.(type) {
	case *CanceledErrorType:
		return 408 // RequestTimeout
	case *UnknownErrorType:
		return 500 // Internal Server Error
	case *InvalidArgumentErrorType:
		return 400 // BadRequest
	case *MalformedErrorType:
		return 400 // BadRequest
	case *DeadlineExceededErrorType:
		return 408 // RequestTimeout
	case *NotFoundErrorType:
		return 404 // Not Found
	case *BadRouteErrorType:
		return 404 // Not Found
	case *AlreadyExistsErrorType:
		return 409 // Conflict
	case *PermissionDeniedErrorType:
		return 403 // Forbidden
	case *UnauthenticatedErrorType:
		return 401 // Unauthorized
	case *ResourceExhaustedErrorType:
		return 429 // RessourceExhausted
	case *FailedPreconditionErrorType:
		return 412 // Precondition Failed
	case *AbortedErrorType:
		return 409 // Conflict
	case *OutOfRangeErrorType:
		return 400 // Bad Request
	case *UnimplementedErrorType:
		return 501 // Not Implemented
	case *InternalErrorType:
		return 500 // Internal Server Error
	case *UnavailableErrorType:
		return 503 // Service Unavailable
	case *DataLossErrorType:
		return 500 // Internal Server Error
	default:
		return 0 // Invalid!
	}
}

//CanceledErrorType indicates the operation was cancelled (typically by the caller).
type CanceledErrorType struct{}

func (e *CanceledErrorType) Error() string { return "canceled" }

//ErrorCanceled constructs a canceled error
func ErrorCanceled(msg string) Error { return NewError(&CanceledErrorType{}, msg) }

//NotFoundErrorType indicates a common NotFound error
type NotFoundErrorType struct{}

func (e *NotFoundErrorType) Error() string { return "not_found" }

//ErrorNotFound constructs a canceled error
func ErrorNotFound(msg string) Error { return NewError(&NotFoundErrorType{}, msg) }

// InvalidArgumentErrorType indicates client specified an invalid argument. It
// indicates arguments that are problematic regardless of the state of the
// system (i.e. a malformed file name, required argument, number out of range,
// etc.).
type InvalidArgumentErrorType struct{}

func (e *InvalidArgumentErrorType) Error() string { return "invalid_argument" }

//ErrorInvalidArgument constructs invalid argument error
func ErrorInvalidArgument(argument string, validationMsg string) Error {
	err := NewError(&InvalidArgumentErrorType{}, fmt.Sprintf("%s %s", argument, validationMsg))
	err = err.WithMeta("argument", argument)
	return err
}

// ErrorRequiredArgument is a more specific constructor for ErrorInvalidArgument.
// Should be used when the argument is required (expected to have a
// non-zero value).
func ErrorRequiredArgument(argument string) Error {
	return ErrorInvalidArgument(argument, "is_required")
}

// InternalErrorType error is an error produced by a downstream dependency of blaze
type InternalErrorType struct {
	err error
}

func (e *InternalErrorType) Error() string {
	if e.err != nil {
		return e.err.Error()
	}
	return "internal"
}

// Unwrap implements the wrappable error
func (e *InternalErrorType) Unwrap() error { return e.err }

// ErrorInternal When some invariants expected by the underlying system
// have been broken. In other words, something bad happened in the library or
// backend service. Do not confuse with HTTP Internal Server Error; an
// Internal error could also happen on the client code, i.e. when parsing a
// server response.
func ErrorInternal(msg string) Error { return NewError(&InternalErrorType{}, msg) }

// ErrorInternalWith  When some invariants expected by the underlying system
// have been broken. In other words, something bad happened in the library or
// backend service. Do not confuse with HTTP Internal Server Error; an
// Internal error could also happen on the client code, i.e. when parsing a
// server response.
// Wraps an other error for more information
func ErrorInternalWith(err error, msg string) Error {
	return NewError(&InternalErrorType{err: err}, msg)
}

//UnknownErrorType For example handling errors raised by APIs that dont return enough error information
type UnknownErrorType struct{}

func (e *UnknownErrorType) Error() string { return "unknown" }

//ErrorUnknown constructs a Unknown error
func ErrorUnknown(msg string) Error { return NewError(&UnknownErrorType{}, msg) }

// MalformedErrorType indicates an error occured while decoding the client's request.
// This means that the message was encoded improperly, or that there is an disagreement in message format
// between client and server
type MalformedErrorType struct{}

func (e *MalformedErrorType) Error() string { return "Malformed" }

//ErrorMalformed constructs a malformed error
func ErrorMalformed(msg string) Error { return NewError(&MalformedErrorType{}, msg) }

//DeadlineExceededErrorType means operation expired before completion. For operations
// that change the state of the system, this error may be returned even if the
// operation has completed successfully (timeout).
type DeadlineExceededErrorType struct{}

func (e *DeadlineExceededErrorType) Error() string { return "deadline_exeeded" }

//ErrorDeadlineExeeded constructs a canceled error
func ErrorDeadlineExeeded(msg string) Error { return NewError(&DeadlineExceededErrorType{}, msg) }

// BadRouteErrorType means that the requested URL path wasn't routable to a blaze
// service and method. This is returned by the generated server, and usually
// shouldn't be returned by applications. Instead, applications should use
// NotFound or Unimplemented.
type BadRouteErrorType struct{}

func (e *BadRouteErrorType) Error() string { return "bad_route" }

//ErrorBadRoute constructs bad route error
func ErrorBadRoute(msg string) Error { return NewError(&BadRouteErrorType{}, msg) }

// AlreadyExistsErrorType means an attempt to create an entity failed because one
// already exists
type AlreadyExistsErrorType struct{}

func (e *AlreadyExistsErrorType) Error() string { return "already_exists" }

//ErrorAlreadyExists constructs a already exists error
func ErrorAlreadyExists(msg string) Error { return NewError(&AlreadyExistsErrorType{}, msg) }

// PermissionDeniedErrorType indicates the caller does not have permission to execute
// the specified operation. It must not be used if the caller cannot be
// identified (Unauthenticated).
type PermissionDeniedErrorType struct{}

func (e *PermissionDeniedErrorType) Error() string { return "permission_denied" }

//ErrorPermissionDenied constructs a permission denied error
func ErrorPermissionDenied(msg string) Error { return NewError(&PermissionDeniedErrorType{}, msg) }

// UnauthenticatedErrorType indicates the request does not have valid authentication
// credentials for the operation.
type UnauthenticatedErrorType struct{}

func (e *UnauthenticatedErrorType) Error() string { return "unauthenticated" }

//ErrorUnauthenticated constructs a unauthenticated error
func ErrorUnauthenticated(msg string) Error { return NewError(&UnauthenticatedErrorType{}, msg) }

// ResourceExhaustedErrorType indicates some resource has been exhausted, perhaps a
// per-user quota, or perhaps the entire file system is out of space.
type ResourceExhaustedErrorType struct{}

func (e *ResourceExhaustedErrorType) Error() string { return "resource_exhausted" }

//ErrorResourceExhausted constructs a resource exhousted error
func ErrorResourceExhausted(msg string) Error { return NewError(&ResourceExhaustedErrorType{}, msg) }

// FailedPreconditionErrorType indicates operation was rejected because the system is
// not in a state required for the operation's execution. For example, doing
// an rmdir operation on a directory that is non-empty, or on a non-directory
// object, or when having conflicting read-modify-write on the same resource.
type FailedPreconditionErrorType struct{}

func (e *FailedPreconditionErrorType) Error() string { return "failed_precondition" }

//ErrorFailedPrecondition constructs a failed precondition error
func ErrorFailedPrecondition(msg string) Error { return NewError(&FailedPreconditionErrorType{}, msg) }

// AbortedErrorType indicates the operation was aborted, typically due to a concurrency
// issue like sequencer check failures, transaction aborts, etc.
type AbortedErrorType struct{}

func (e *AbortedErrorType) Error() string { return "aborted" }

//ErrorAborted constructs a aborted error
func ErrorAborted(msg string) Error { return NewError(&AbortedErrorType{}, msg) }

// OutOfRangeErrorType means operation was attempted past the valid range. For example,
// seeking or reading past end of a paginated collection.
//
// Unlike InvalidArgument, this error indicates a problem that may be fixed if
// the system state changes (i.e. adding more items to the collection).
//
// There is a fair bit of overlap between FailedPrecondition and OutOfRange.
// We recommend using OutOfRange (the more specific error) when it applies so
// that callers who are iterating through a space can easily look for an
// OutOfRange error to detect when they are done.
type OutOfRangeErrorType struct{}

func (e *OutOfRangeErrorType) Error() string { return "out_of_range" }

//ErrorOutOfRange constructs a out of range error
func ErrorOutOfRange(msg string) Error { return NewError(&OutOfRangeErrorType{}, msg) }

// UnimplementedErrorType indicates operation is not implemented or not
// supported/enabled in this service.
type UnimplementedErrorType struct{}

func (e *UnimplementedErrorType) Error() string { return "unimplemented" }

//ErrorUnimplemented constructs an unimplemented error
func ErrorUnimplemented(msg string) Error { return NewError(&UnimplementedErrorType{}, msg) }

// UnavailableErrorType indicates the service is currently unavailable. This is a most
// likely a transient condition and may be corrected by retrying with a
// backoff.
type UnavailableErrorType struct{}

func (e *UnavailableErrorType) Error() string { return "unavailable" }

//ErrorUnavailable constructs an unavailable error
func ErrorUnavailable(msg string) Error { return NewError(&UnavailableErrorType{}, msg) }

// DataLossErrorType indicates unrecoverable data loss or corruption.
type DataLossErrorType struct{}

func (e *DataLossErrorType) Error() string { return "data_loss" }

//ErrorDataLoss constructs a data loss error
func ErrorDataLoss(msg string) Error { return NewError(&DataLossErrorType{}, msg) }
