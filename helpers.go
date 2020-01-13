/*
  File: \helpers.go
  Created Date: Thursday, January 9th 2020, 3:12:21 pm
  Author: Ralf Mueller
  -----
  Last Modified:
  Modified By:
  -----
  Copyright (c) 2020 Ralf Mueller


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
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-logr/logr"
)

//ServerBadRouteError is used when the blaze server cannot route a request
func ServerBadRouteError(msg string, method, url string) error {
	err := ErrorBadRoute(msg)
	err = err.WithMeta("blaze_invalid_route", method+" "+url)
	return err
}

// ServerInvalidRequestError is used when the server is called with an invalid argument
func ServerInvalidRequestError(argument string, validationMsg string, method, url string) error {
	err := ErrorInvalidArgument(argument, validationMsg)
	err = err.WithMeta("blaze_invalid_route", method+" "+url)
	return err
}

// ServerWriteError writes Blaze errors in the response and triggers hooks.
func ServerWriteError(ctx context.Context, resp http.ResponseWriter, err error, log logr.Logger) {
	// Non-blaze errors are wrapped as Internal (default)
	blerr, ok := err.(Error)
	if !ok {
		blerr = ErrorInternalWith(err, "")
	}

	statusCode := ServerHTTPStatusFromErrorType(blerr)

	respBody := marshalErrorToJSON(blerr)

	resp.Header().Set("Content-Type", "application/json") // Error responses are always JSON
	resp.Header().Set("Content-Length", strconv.Itoa(len(respBody)))
	resp.WriteHeader(statusCode) // set HTTP status code and send response

	_, writeErr := resp.Write(respBody)
	if writeErr != nil {
		blerr := ErrorInternalWith(writeErr, "resp write failed")
		log.Error(blerr, "")
		_ = writeErr
	}
}

// ServerEnsurePanicResponses esure panic responses
func ServerEnsurePanicResponses(ctx context.Context, resp http.ResponseWriter, log logr.Logger) {
	if r := recover(); r != nil {
		// Wrap the panic as an error so it can be passed to error hooks.
		// The original error is accessible from error hooks, but not visible in the response.
		// After hooks are implemented that is :)
		err := errFromPanic(r)
		blerr := ErrorInternalWith(err, "Internal service panic")
		// Actually write the error
		ServerWriteError(ctx, resp, blerr, log)
		// If possible, flush the error to the wire.
		f, ok := resp.(http.Flusher)
		if ok {
			f.Flush()
		}

		panic(r)
	}
}

// JSON serialization for errors
type blazeJSON struct {
	Code string            `json:"code"`
	Msg  string            `json:"msg"`
	Meta map[string]string `json:"meta,omitempty"`
}

// marshalErrorToJSON returns JSON from a blaze.Error, that can be used as HTTP error response body.
// If serialization fails, it will use a descriptive Internal error instead.
func marshalErrorToJSON(blerr Error) []byte {
	// make sure that msg is not too large
	msg := blerr.Msg()
	if len(msg) > 1e6 {
		msg = msg[:1e6]
	}

	tj := blazeJSON{
		Code: strconv.Itoa(ServerHTTPStatusFromErrorType(blerr)),
		Msg:  msg,
		Meta: blerr.MetaMap(),
	}

	buf, err := json.Marshal(&tj)
	if err != nil {
		buf = []byte("{\"type\": \"" + "blaze.Internal" + "\", \"msg\": \"There was an error but it could not be serialized into JSON\"}") // fallback
	}

	return buf
}

func errFromPanic(p interface{}) error {
	if err, ok := p.(error); ok {
		return err
	}
	return fmt.Errorf("panic: %v", p)
}

// WithoutRedirects makes sure that the POST request can not be redirected.
// The standard library will, by default, redirect requests (including POSTs) if it gets a 302 or
// 303 response, and also 301s in go1.8. It redirects by making a second request, changing the
// method to GET and removing the body. This produces very confusing error messages, so instead we
// set a redirect policy that always errors. This stops Go from executing the redirect.
//
// We have to be a little careful in case the user-provided http.Client has its own CheckRedirect
// policy - if so, we'll run through that policy first.
//
// Because this requires modifying the http.Client, we make a new copy of the client and return it.
func WithoutRedirects(in *http.Client) *http.Client {
	copy := *in
	copy.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if in.CheckRedirect != nil {
			// Run the input's redirect if it exists, in case it has side effects, but ignore any error it
			// returns, since we want to use ErrUseLastResponse.
			err := in.CheckRedirect(req, via)
			_ = err // Silly, but this makes sure generated code passes errcheck -blank, which some people use.
		}
		return http.ErrUseLastResponse
	}
	return &copy
}
