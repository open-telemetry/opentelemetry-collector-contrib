// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"errors"
	"net/http"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/xray/types"
	"github.com/aws/smithy-go"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"github.com/stretchr/testify/assert"
)

func TestRecordConnectionError(t *testing.T) {
	type testParameters struct {
		input error
		want  func() types.TelemetryRecord
	}
	testCases := []testParameters{
		{
			input: &smithyhttp.ResponseError{
				Response: &smithyhttp.Response{
					Response: &http.Response{
						StatusCode: http.StatusInternalServerError,
					},
				},
			},
			want: func() types.TelemetryRecord {
				record := NewRecord()
				record.BackendConnectionErrors.HTTPCode5XXCount = aws.Int32(1)
				return record
			},
		},
		{
			input: &smithyhttp.ResponseError{
				Response: &smithyhttp.Response{
					Response: &http.Response{
						StatusCode: http.StatusBadRequest,
					},
				},
			},
			want: func() types.TelemetryRecord {
				record := NewRecord()
				record.BackendConnectionErrors.HTTPCode4XXCount = aws.Int32(1)
				return record
			},
		},
		{
			input: &smithyhttp.ResponseError{
				Response: &smithyhttp.Response{
					Response: &http.Response{
						StatusCode: http.StatusFound,
					},
				},
			},
			want: func() types.TelemetryRecord {
				record := NewRecord()
				record.BackendConnectionErrors.OtherCount = aws.Int32(1)
				return record
			},
		},
		{
			input: &smithy.GenericAPIError{Code: ErrCodeResponseTimeout, Message: ""},
			want: func() types.TelemetryRecord {
				record := NewRecord()
				record.BackendConnectionErrors.TimeoutCount = aws.Int32(1)
				return record
			},
		},
		{
			input: &smithy.GenericAPIError{Code: ErrCodeRequestError, Message: ""},
			want: func() types.TelemetryRecord {
				record := NewRecord()
				record.BackendConnectionErrors.UnknownHostCount = aws.Int32(1)
				return record
			},
		},
		{
			input: &smithy.DeserializationError{},
			want: func() types.TelemetryRecord {
				record := NewRecord()
				record.BackendConnectionErrors.OtherCount = aws.Int32(1)
				return record
			},
		},
		{
			input: errors.New("test"),
			want: func() types.TelemetryRecord {
				record := NewRecord()
				record.BackendConnectionErrors.OtherCount = aws.Int32(1)
				return record
			},
		},
		{
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
