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
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/golang/protobuf/ptypes/duration"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestHttpError_GRPCStatus(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantCode   codes.Code
	}{
		{
			name:       "Bad Request",
			statusCode: http.StatusBadRequest,
			wantCode:   codes.InvalidArgument,
		},
		{
			name:       "Forbidden",
			statusCode: http.StatusForbidden,
			wantCode:   codes.Unauthenticated,
		},
		{
			name:       "Internal",
			statusCode: http.StatusInternalServerError,
			wantCode:   codes.DataLoss,
		},
		{
			name:       "Teapot",
			statusCode: http.StatusTeapot,
			wantCode:   codes.DataLoss,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			httpResponse := responseOf(test.statusCode)
			httpError := &httpError{response: httpResponse, err: errors.New("uh oh")}
			got := httpError.GRPCStatus()
			assert.Equal(t, test.wantCode, got.Code())
			assert.Equal(t, httpResponse.Status, got.Message())
		})
	}

	t.Run("Retry-After Bad Header", func(t *testing.T) {
		response := responseOf(http.StatusTooManyRequests)
		response.Header.Add("Retry-After", "foo")
		assert.Equal(
			t,
			status.New(codes.Unavailable, http.StatusText(http.StatusTooManyRequests)),
			(&httpError{response: response}).GRPCStatus(),
		)
	})

	t.Run("Retry-After Good Header", func(t *testing.T) {
		response := responseOf(http.StatusTooManyRequests)
		response.Header.Add("Retry-After", "10")
		expectedMessage := &errdetails.RetryInfo{RetryDelay: &duration.Duration{Seconds: 10}}
		expectedStatus, _ := status.New(codes.Unavailable, http.StatusText(http.StatusTooManyRequests)).WithDetails(expectedMessage)
		assert.Equal(
			t,
			expectedStatus,
			newHTTPError(response).GRPCStatus(),
		)
	})
}

func TestHttpErrorWrapGeneratesThrottleRetryOn429StatusCode(t *testing.T) {
	expected := exporterhelper.NewThrottleRetry(
		fmt.Errorf("new relic HTTP call failed. Status Code: 429"),
		time.Duration(10)*time.Second)

	response := responseOf(http.StatusTooManyRequests)
	response.Header.Add("Retry-After", "10")

	httpError := newHTTPError(response)
	wrappedErr := httpError.Wrap()
	actual := errors.Unwrap(wrappedErr)

	assert.EqualValues(t, expected, actual)
}

func TestHttpError_Error(t *testing.T) {
	httpError := newHTTPError(responseOf(http.StatusTeapot))
	assert.Equal(t, httpError.Error(), "new relic HTTP call failed. Status Code: 418")
}

func responseOf(statusCode int) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Status:     http.StatusText(statusCode),
		Header:     http.Header{},
	}
}

type myError struct {
	temporary bool
}

func (e *myError) Error() string   { return "uh oh" }
func (e *myError) Temporary() bool { return e.temporary }

func TestUrlError_GRPCStatus(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode codes.Code
	}{
		{
			name: "Temporary",
			err: &url.Error{
				Op:  "Get",
				Err: &myError{temporary: true},
			},
			wantCode: codes.DataLoss,
		},
		{
			name: "No-Retry",
			err: &url.Error{
				Op:  "Get",
				Err: &myError{temporary: false},
			},
			wantCode: codes.Internal,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := &urlError{err: test.err}
			got := err.GRPCStatus()
			assert.Equal(t, test.wantCode, got.Code())
			assert.Equal(t, "Get \"\": uh oh", got.Message())
		})
	}
}

func TestUrlError_Error(t *testing.T) {
	netErr := &url.Error{
		Op:  "Get",
		Err: errors.New("uh oh"),
	}
	err := &urlError{err: netErr}
	assert.Equal(t, netErr.Error(), err.Error())
}

func TestUrlError_Unwrap(t *testing.T) {
	netErr := &url.Error{
		Op:  "Get",
		Err: errors.New("uh oh"),
	}
	err := &urlError{err: netErr}
	assert.Equal(t, netErr, err.Unwrap())
}

func TestFromGrpcErrorHttpError(t *testing.T) {
	httpError := newHTTPError(responseOf(http.StatusTeapot))
	_, ok := status.FromError(httpError)
	assert.True(t, ok)
}

func TestFromGrpcErrorUrlError(t *testing.T) {
	netErr := &url.Error{
		Op:  "Get",
		Err: errors.New("uh oh"),
	}
	err := &urlError{err: netErr}
	_, ok := status.FromError(err)
	assert.True(t, ok)
}
