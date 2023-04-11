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
