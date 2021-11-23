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
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/golang/protobuf/ptypes/duration"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	grpcStatus "google.golang.org/grpc/status"
)

type urlError struct {
	err error
}

func (e *urlError) Error() string { return e.err.Error() }
func (e *urlError) Unwrap() error { return e.err }

func (e *urlError) GRPCStatus() *grpcStatus.Status {
	urlError := e.err.(*url.Error)
	// If error is temporary, return retryable DataLoss code
	if urlError.Temporary() {
		return grpcStatus.New(codes.DataLoss, urlError.Error())
	}
	// Else, return non-retryable Internal code
	return grpcStatus.New(codes.Internal, urlError.Error())
}

func (e *urlError) IsPermanent() bool {
	urlError := e.err.(*url.Error)
	return !urlError.Temporary()
}

func (e *urlError) Wrap() error {
	var out error
	out = e
	if e.IsPermanent() {
		out = consumererror.NewPermanent(out)
	}
	return out
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
	http.StatusBadGateway:                  codes.DataLoss,
	http.StatusServiceUnavailable:          codes.DataLoss,
}

var retryableHTTPCodes = map[int]struct{}{
	http.StatusRequestTimeout:      {},
	http.StatusTooManyRequests:     {},
	http.StatusInternalServerError: {},
	http.StatusBadGateway:          {},
	http.StatusServiceUnavailable:  {},
}

type httpError struct {
	err      error
	response *http.Response
}

func (e *httpError) Unwrap() error { return e.err }
func (e *httpError) Error() string { return e.err.Error() }

func (e *httpError) GRPCStatus() *grpcStatus.Status {
	mapEntry, ok := httpGrpcMapping[e.response.StatusCode]
	// If no explicit mapping exists, return retryable DataLoss code
	if !ok {
		return grpcStatus.New(codes.DataLoss, e.response.Status)
	}
	// The OTLP spec uses the Unavailable code to signal backpressure to the client
	// If the http status maps to Unavailable, attempt to extract and communicate retry info to the client
	retrySeconds := int64(e.ThrottleDelay().Seconds())
	if retrySeconds > 0 {
		message := &errdetails.RetryInfo{RetryDelay: &duration.Duration{Seconds: retrySeconds}}
		status, statusErr := grpcStatus.New(codes.Unavailable, e.response.Status).WithDetails(message)
		if statusErr == nil {
			return status
		}
	}

	// Generate an error with the mapped code, and a message containing the server's response status string
	return grpcStatus.New(mapEntry, e.response.Status)
}

func newHTTPError(response *http.Response) *httpError {
	return &httpError{
		err:      fmt.Errorf("new relic HTTP call failed. Status Code: %d", response.StatusCode),
		response: response,
	}
}

func (e *httpError) IsPermanent() bool {
	if _, ok := retryableHTTPCodes[e.response.StatusCode]; ok {
		return false
	}
	return true
}

func (e *httpError) ThrottleDelay() time.Duration {
	if e.response.StatusCode == http.StatusTooManyRequests {
		retryAfter := e.response.Header.Get("Retry-After")
		if retrySeconds, err := strconv.ParseInt(retryAfter, 10, 64); err == nil {
			return time.Duration(retrySeconds) * time.Second
		}
	}
	return 0
}

func (e *httpError) Wrap() error {
	if e.IsPermanent() {
		out := consumererror.NewPermanent(e.err)
		return &httpError{err: out, response: e.response}
	} else if delay := e.ThrottleDelay(); delay > 0 {
		// NOTE: a retry-after header means that the error is not
		// permanent (by definition). Here, we use an else if to
		// optimize the branch conditions. If an error is permanent,
		// there's no reason to check for a throttle delay.
		out := exporterhelper.NewThrottleRetry(e.err, delay)
		return &httpError{err: out, response: e.response}
	} else {
		return e
	}
}
