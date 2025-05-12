// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"errors"
	"net/http"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/service/xray/types"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"github.com/stretchr/testify/assert"
)

func TestRecordConnectionError(t *testing.T) {
	tests := []struct {
		name  string
		input error
		want  func() types.TelemetryRecord
	}{
		{
			name: "5xx error",
			input: &awshttp.ResponseError{
				ResponseError: &smithyhttp.ResponseError{
					Response: &smithyhttp.Response{
						Response: &http.Response{
							StatusCode: http.StatusInternalServerError,
						},
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
			name: "4xx error",
			input: &awshttp.ResponseError{
				ResponseError: &smithyhttp.ResponseError{
					Response: &smithyhttp.Response{
						Response: &http.Response{
							StatusCode: http.StatusBadRequest,
						},
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
			name: "Other error (302)",
			input: &awshttp.ResponseError{
				ResponseError: &smithyhttp.ResponseError{
					Response: &smithyhttp.Response{
						Response: &http.Response{
							StatusCode: http.StatusFound,
						},
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
			name:  "response timeout",
			input: &awshttp.ResponseTimeoutError{},
			want: func() types.TelemetryRecord {
				record := NewRecord()
				record.BackendConnectionErrors.TimeoutCount = aws.Int32(1)
				return record
			},
		},
		{
			name:  "request error",
			input: &smithyhttp.RequestSendError{},
			want: func() types.TelemetryRecord {
				record := NewRecord()
				record.BackendConnectionErrors.UnknownHostCount = aws.Int32(1)
				return record
			},
		},
		{
			name:  "other error (test)",
			input: errors.New("test"),
			want: func() types.TelemetryRecord {
				record := NewRecord()
				record.BackendConnectionErrors.OtherCount = aws.Int32(1)
				return record
			},
		},
		{
			name:  "no error",
			input: nil,
			want:  NewRecord,
		},
	}

	recorder := NewRecorder()
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder.RecordConnectionError(test.input)
			snapshot := recorder.Rotate()
			assert.Equal(t, test.want().BackendConnectionErrors, snapshot.BackendConnectionErrors)
		})
	}
}
