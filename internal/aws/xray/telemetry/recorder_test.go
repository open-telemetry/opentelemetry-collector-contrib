// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"errors"
	"net/http"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/xray/types"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/stretchr/testify/assert"
)

func TestRecordConnectionError(t *testing.T) {
	type testParameters struct {
		input error
		want  func() types.TelemetryRecord
	}
	testCases := []testParameters{
		{
			input: awserr.NewRequestFailure(nil, http.StatusInternalServerError, ""),
			want: func() types.TelemetryRecord {
				record := NewRecord()
				record.BackendConnectionErrors.HTTPCode5XXCount = aws.Int32(1)
				return record
			},
		},
		{
			input: awserr.NewRequestFailure(nil, http.StatusBadRequest, ""),
			want: func() types.TelemetryRecord {
				record := NewRecord()
				record.BackendConnectionErrors.HTTPCode4XXCount = aws.Int32(1)
				return record
			},
		},
		{
			input: awserr.NewRequestFailure(nil, http.StatusFound, ""),
			want: func() types.TelemetryRecord {
				record := NewRecord()
				record.BackendConnectionErrors.OtherCount = aws.Int32(1)
				return record
			},
		},
		{
			input: awserr.New(request.ErrCodeResponseTimeout, "", nil),
			want: func() types.TelemetryRecord {
				record := NewRecord()
				record.BackendConnectionErrors.TimeoutCount = aws.Int32(1)
				return record
			},
		},
		{
			input: awserr.New(request.ErrCodeRequestError, "", nil),
			want: func() types.TelemetryRecord {
				record := NewRecord()
				record.BackendConnectionErrors.UnknownHostCount = aws.Int32(1)
				return record
			},
		},
		{
			input: awserr.New(request.ErrCodeSerialization, "", nil),
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
		assert.Equal(t, testCase.want().BackendConnectionErrors, snapshot.BackendConnectionErrors)
	}
}
