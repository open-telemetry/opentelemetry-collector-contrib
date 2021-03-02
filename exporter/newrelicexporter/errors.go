// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package newrelicexporter

import (
	"net/http"
	"net/url"
	"strconv"

	"github.com/golang/protobuf/ptypes/duration"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	grpcStatus "google.golang.org/grpc/status"
)

type urlError struct {
	Err error
}

func (e *urlError) Error() string { return e.Err.Error() }
func (e *urlError) Unwrap() error { return e.Err }

func (e *urlError) GRPCStatus() *grpcStatus.Status {
	urlError := e.Err.(*url.Error)
	// If error is temporary, return retryable DataLoss code
	if urlError.Temporary() {
		return grpcStatus.New(codes.DataLoss, urlError.Error())
	}
	// Else, return non-retryable Internal code
	return grpcStatus.New(codes.Internal, urlError.Error())
}

// Explicit mapping for the error status codes describe by the trace API:
// https://docs.newrelic.com/docs/understand-dependencies/distributed-tracing/trace-api/trace-api-general-requirements-limits#response-validation
var httpGrpcMapping = map[int]codes.Code{
	http.StatusBadRequest:                  codes.InvalidArgument,
	http.StatusForbidden:                   codes.Unauthenticated,
	http.StatusNotFound:                    codes.NotFound,
	http.StatusMethodNotAllowed:            codes.InvalidArgument,
	http.StatusRequestTimeout:              codes.DeadlineExceeded,
	http.StatusLengthRequired:              codes.InvalidArgument,
	http.StatusRequestEntityTooLarge:       codes.InvalidArgument,
	http.StatusRequestURITooLong:           codes.InvalidArgument,
	http.StatusUnsupportedMediaType:        codes.InvalidArgument,
	http.StatusTooManyRequests:             codes.Unavailable,
	http.StatusRequestHeaderFieldsTooLarge: codes.InvalidArgument,
	http.StatusInternalServerError:         codes.DataLoss,
}

type httpError struct {
	Response *http.Response
}

func (e *httpError) Error() string { return "New Relic HTTP call failed" }

func (e *httpError) GRPCStatus() *grpcStatus.Status {
	mapEntry, ok := httpGrpcMapping[e.Response.StatusCode]
	// If no explicit mapping exists, return retryable DataLoss code
	if !ok {
		return grpcStatus.New(codes.DataLoss, e.Response.Status)
	}
	// The OTLP spec uses the Unavailable code to signal backpressure to the client
	// If the http status maps to Unavailable, attempt to extract and communicate retry info to the client
	if mapEntry == codes.Unavailable {
		retryAfter := e.Response.Header.Get("Retry-After")
		retrySeconds, err := strconv.ParseInt(retryAfter, 10, 64)
		if err == nil {
			message := &errdetails.RetryInfo{RetryDelay: &duration.Duration{Seconds: retrySeconds}}
			status, statusErr := grpcStatus.New(codes.Unavailable, e.Response.Status).WithDetails(message)
			if statusErr == nil {
				return status
			}
		}
	}

	// Generate an error with the mapped code, and a message containing the server's response status string
	return grpcStatus.New(mapEntry, e.Response.Status)
}
