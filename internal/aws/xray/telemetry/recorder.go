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

package telemetry // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray/telemetry"

import (
	"errors"
	"sync/atomic"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/xray"
)

type Recorder interface {
	// Rotate the current record by swapping it out with a new one. Returns
	// the rotated record.
	Rotate() *xray.TelemetryRecord
	// HasRecording indicates whether any of the record functions were called
	// with the current record.
	HasRecording() bool
	// RecordSegmentsReceived adds the count to the current record.
	RecordSegmentsReceived(count int)
	// RecordSegmentsSent adds the count to the current record.
	RecordSegmentsSent(count int)
	// RecordSegmentsSpillover adds the count to the current record.
	RecordSegmentsSpillover(count int)
	// RecordSegmentsRejected adds the count to the current record.
	RecordSegmentsRejected(count int)
	// RecordConnectionError categorizes the error and increments the count by one
	// for the current record.
	RecordConnectionError(err error)
}

type telemetryRecorder struct {
	// record is the pointer to the count metrics for the current period.
	record *xray.TelemetryRecord
	// hasRecording is set to true when any count is updated. Indicates
	// that telemetry data is available.
	hasRecording *atomic.Bool
}

// NewRecorder creates a new Recorder with a default interval and queue size.
func NewRecorder() Recorder {
	return &telemetryRecorder{
		record:       NewRecord(),
		hasRecording: &atomic.Bool{},
	}
}

// NewRecord creates a new xray.TelemetryRecord with all of its fields initialized
// and set to 0.
func NewRecord() *xray.TelemetryRecord {
	return &xray.TelemetryRecord{
		SegmentsReceivedCount:  aws.Int64(0),
		SegmentsRejectedCount:  aws.Int64(0),
		SegmentsSentCount:      aws.Int64(0),
		SegmentsSpilloverCount: aws.Int64(0),
		BackendConnectionErrors: &xray.BackendConnectionErrors{
			HTTPCode4XXCount:       aws.Int64(0),
			HTTPCode5XXCount:       aws.Int64(0),
			ConnectionRefusedCount: aws.Int64(0),
			OtherCount:             aws.Int64(0),
			TimeoutCount:           aws.Int64(0),
			UnknownHostCount:       aws.Int64(0),
		},
	}
}

func (tr *telemetryRecorder) HasRecording() bool {
	return tr.hasRecording.Load()
}

// Rotate the current record and swaps it out with a new record.
// Sets the timestamp and returns the old record.
func (tr *telemetryRecorder) Rotate() *xray.TelemetryRecord {
	snapshot := NewRecord()
	snapshot.SetSegmentsSentCount(atomic.SwapInt64(tr.record.SegmentsSentCount, 0))
	snapshot.SetSegmentsReceivedCount(atomic.SwapInt64(tr.record.SegmentsReceivedCount, 0))
	snapshot.SetSegmentsRejectedCount(atomic.SwapInt64(tr.record.SegmentsRejectedCount, 0))
	snapshot.SetSegmentsSpilloverCount(atomic.SwapInt64(tr.record.SegmentsSpilloverCount, 0))
	snapshot.BackendConnectionErrors.SetHTTPCode4XXCount(atomic.SwapInt64(tr.record.BackendConnectionErrors.HTTPCode4XXCount, 0))
	snapshot.BackendConnectionErrors.SetHTTPCode5XXCount(atomic.SwapInt64(tr.record.BackendConnectionErrors.HTTPCode5XXCount, 0))
	snapshot.BackendConnectionErrors.SetTimeoutCount(atomic.SwapInt64(tr.record.BackendConnectionErrors.TimeoutCount, 0))
	snapshot.BackendConnectionErrors.SetConnectionRefusedCount(atomic.SwapInt64(tr.record.BackendConnectionErrors.ConnectionRefusedCount, 0))
	snapshot.BackendConnectionErrors.SetUnknownHostCount(atomic.SwapInt64(tr.record.BackendConnectionErrors.UnknownHostCount, 0))
	snapshot.BackendConnectionErrors.SetOtherCount(atomic.SwapInt64(tr.record.BackendConnectionErrors.OtherCount, 0))
	snapshot.SetTimestamp(time.Now())
	tr.hasRecording.Store(false)
	return snapshot
}

func (tr *telemetryRecorder) RecordSegmentsReceived(count int) {
	atomic.AddInt64(tr.record.SegmentsReceivedCount, int64(count))
	tr.hasRecording.Store(true)
}

func (tr *telemetryRecorder) RecordSegmentsSent(count int) {
	atomic.AddInt64(tr.record.SegmentsSentCount, int64(count))
	tr.hasRecording.Store(true)
}

func (tr *telemetryRecorder) RecordSegmentsSpillover(count int) {
	atomic.AddInt64(tr.record.SegmentsSpilloverCount, int64(count))
	tr.hasRecording.Store(true)
}

func (tr *telemetryRecorder) RecordSegmentsRejected(count int) {
	atomic.AddInt64(tr.record.SegmentsRejectedCount, int64(count))
	tr.hasRecording.Store(true)
}

func (tr *telemetryRecorder) RecordConnectionError(err error) {
	if err == nil {
		return
	}
	var requestFailure awserr.RequestFailure
	if ok := errors.As(err, &requestFailure); ok {
		switch requestFailure.StatusCode() / 100 {
		case 5:
			atomic.AddInt64(tr.record.BackendConnectionErrors.HTTPCode5XXCount, 1)
		case 4:
			atomic.AddInt64(tr.record.BackendConnectionErrors.HTTPCode4XXCount, 1)
		default:
			atomic.AddInt64(tr.record.BackendConnectionErrors.OtherCount, 1)
		}
	} else {
		var awsError awserr.Error
		if ok = errors.As(err, &awsError); ok {
			switch awsError.Code() {
			case request.ErrCodeResponseTimeout:
				atomic.AddInt64(tr.record.BackendConnectionErrors.TimeoutCount, 1)
			case request.ErrCodeRequestError:
				atomic.AddInt64(tr.record.BackendConnectionErrors.UnknownHostCount, 1)
			default:
				atomic.AddInt64(tr.record.BackendConnectionErrors.OtherCount, 1)
			}
		} else {
			atomic.AddInt64(tr.record.BackendConnectionErrors.OtherCount, 1)
		}
	}
	tr.hasRecording.Store(true)
}
