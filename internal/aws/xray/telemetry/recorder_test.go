// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/xray/types"
	"github.com/aws/smithy-go"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"github.com/stretchr/testify/assert"
)

// Define a test API error type that implements smithy.APIError interface
type apiError struct {
	Code    string
	Message string
	Fault   smithy.ErrorFault
}

func (e *apiError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *apiError) ErrorCode() string {
	return e.Code
}

func (e *apiError) ErrorMessage() string {
	return e.Message
}

func (e *apiError) ErrorFault() smithy.ErrorFault {
	return e.Fault
}

// Helper function to create HTTP response errors
func newHTTPResponseError(code int, message string) error {
	var apiErr *apiError

	switch {
	case code >= 500:
		apiErr = &apiError{
			Code:    fmt.Sprintf("%d", code),
			Message: message,
			Fault:   smithy.FaultServer,
		}
	case code >= 400:
		apiErr = &apiError{
			Code:    fmt.Sprintf("%d", code),
			Message: message,
			Fault:   smithy.FaultClient,
		}
	default:
		apiErr = &apiError{
			Code:    fmt.Sprintf("%d", code),
			Message: message,
			Fault:   smithy.FaultUnknown,
		}
	}

	// Wrap in HTTP response error
	return &smithyhttp.ResponseError{
		Response: &smithyhttp.Response{
			Response: &http.Response{
				StatusCode: code,
			},
		},
		Err: apiErr,
	}
}

func TestRecordConnectionError(t *testing.T) {
	type testParameters struct {
		input error
		want  func() *types.TelemetryRecord
	}

	testCases := []testParameters{
		{
			// Test 500 error
			input: newHTTPResponseError(http.StatusInternalServerError, "Internal Server Error"),
			want: func() *types.TelemetryRecord {
				record := NewRecord()
				record.BackendConnectionErrors.HTTPCode5XXCount = aws.Int32(1)
				return record
			},
		},
		{
			// Test 400 error
			input: newHTTPResponseError(http.StatusBadRequest, "Bad Request"),
			want: func() *types.TelemetryRecord {
				record := NewRecord()
				record.BackendConnectionErrors.HTTPCode4XXCount = aws.Int32(1)
				return record
			},
		},
		{
			// Test 302 error
			input: newHTTPResponseError(http.StatusFound, "Found"),
			want: func() *types.TelemetryRecord {
				record := NewRecord()
				record.BackendConnectionErrors.OtherCount = aws.Int32(1)
				return record
			},
		},
		{
			// Test timeout error
			input: &apiError{
				Code:    "RequestTimeout",
				Message: "Request timed out",
				Fault:   smithy.FaultServer,
			},
			want: func() *types.TelemetryRecord {
				record := NewRecord()
				record.BackendConnectionErrors.TimeoutCount = aws.Int32(1)
				return record
			},
		},
		{
			// Test unknown host error
			input: &apiError{
				Code:    "UnknownHostException",
				Message: "No such host",
				Fault:   smithy.FaultClient,
			},
			want: func() *types.TelemetryRecord {
				record := NewRecord()
				record.BackendConnectionErrors.UnknownHostCount = aws.Int32(1)
				return record
			},
		},
		{
			// Test serialization error
			input: &apiError{
				Code:    "SerializationError",
				Message: "Failed to serialize request",
				Fault:   smithy.FaultClient,
			},
			want: func() *types.TelemetryRecord {
				record := NewRecord()
				record.BackendConnectionErrors.OtherCount = aws.Int32(1)
				return record
			},
		},
		{
			// Test generic error
			input: errors.New("test"),
			want: func() *types.TelemetryRecord {
				record := NewRecord()
				record.BackendConnectionErrors.OtherCount = aws.Int32(1)
				return record
			},
		},
		{
			// Test nil error
			input: nil,
			want:  NewRecord,
		},
	}

	recorder := NewRecorder()
	for _, testCase := range testCases {
		recorder.RecordConnectionError(testCase.input)
		snapshot := recorder.Rotate()
		assert.EqualValues(t, testCase.want().BackendConnectionErrors, snapshot.BackendConnectionErrors)
	}
}