// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"errors"
	"net/http"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/xray"
	"github.com/stretchr/testify/assert"
)

func TestRecordConnectionError(t *testing.T) {
	type testParameters struct {
		input error
		want  func() *xray.TelemetryRecord
	}
	testCases := []testParameters{
		{
			input: awserr.NewRequestFailure(nil, http.StatusInternalServerError, ""),
			want: func() *xray.TelemetryRecord {
				record := NewRecord()
				record.BackendConnectionErrors.HTTPCode5XXCount = aws.Int64(1)
				return record
			},
		},
		{
			input: awserr.NewRequestFailure(nil, http.StatusBadRequest, ""),
			want: func() *xray.TelemetryRecord {
				record := NewRecord()
				record.BackendConnectionErrors.HTTPCode4XXCount = aws.Int64(1)
				return record
			},
		},
		{
			input: awserr.NewRequestFailure(nil, http.StatusFound, ""),
			want: func() *xray.TelemetryRecord {
				record := NewRecord()
				record.BackendConnectionErrors.OtherCount = aws.Int64(1)
				return record
			},
		},
		{
			input: awserr.New(request.ErrCodeResponseTimeout, "", nil),
			want: func() *xray.TelemetryRecord {
				record := NewRecord()
				record.BackendConnectionErrors.TimeoutCount = aws.Int64(1)
				return record
			},
		},
		{
			input: awserr.New(request.ErrCodeRequestError, "", nil),
			want: func() *xray.TelemetryRecord {
				record := NewRecord()
				record.BackendConnectionErrors.UnknownHostCount = aws.Int64(1)
				return record
			},
		},
		{
			input: awserr.New(request.ErrCodeSerialization, "", nil),
			want: func() *xray.TelemetryRecord {
				record := NewRecord()
				record.BackendConnectionErrors.OtherCount = aws.Int64(1)
				return record
			},
		},
		{
			input: errors.New("test"),
			want: func() *xray.TelemetryRecord {
				record := NewRecord()
				record.BackendConnectionErrors.OtherCount = aws.Int64(1)
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
