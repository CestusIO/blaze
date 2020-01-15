/*
  File: \blaze.go
  Created Date: Thursday, January 9th 2020, 3:15:06 pm
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
	"net/http"

	"github.com/go-chi/chi"
)

// Server defines the interface of blazeserver
type Server interface {
	Mux() *chi.Mux
	MountPath() string
}

// ServerOption is a functional option for extending a Blaze server.
type ServerOption func(*ServerOptions)

// ServerOptions encapsulate the configurable parameters on a Blaze server.
type ServerOptions struct {
	// Uses a specific mux instead of chi.NewRouter()
	Mux *chi.Mux
	// Whether to render enum values as integers, as opposed to string values.
	JSONEnumsAsInts bool
	// Whether to render fields with zero values.
	JSONEmitDefaults bool
}

// WithMux allows to set the chi mux to use by a server
func WithMux(mux *chi.Mux) ServerOption {
	return func(o *ServerOptions) {
		o.Mux = mux
	}
}

// WithJSONEnumsAsInts makes enums be rendered as ints instead of strings
func WithJSONEnumsAsInts(v bool) ServerOption {
	return func(o *ServerOptions) {
		o.JSONEnumsAsInts = v
	}
}

// WithJSONEmitDefaults makes JSON structs to render fields even if they are empty or the default value
func WithJSONEmitDefaults(v bool) ServerOption {
	return func(o *ServerOptions) {
		o.JSONEmitDefaults = v
	}
}

// ClientOption is a functional option for extending a Blaze client.
type ClientOption func(*ClientOptions)

// ClientOptions encapsulate the configurable parameters on a Blaze client.
type ClientOptions struct {
}

// HTTPClient is the interface used by generated clients to send HTTP requests.
// It is fulfilled by *(net/http).Client, which is sufficient for most users.
// Users can provide their own implementation for special retry policies.
//
// HTTPClient implementations should not follow redirects. Redirects are
// automatically disabled if *(net/http).Client is passed to client
// constructors. See the withoutRedirects function in this file for more
// details.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}
