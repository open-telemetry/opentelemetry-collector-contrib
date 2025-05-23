// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package telemetry // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray/telemetry"

import (
	"errors"
	"sync/atomic"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/service/xray/types"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

type Recorder interface {
	// Rotate the current record by swapping it out with a new one. Returns
	// the rotated record.
	Rotate() types.TelemetryRecord
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
	record types.TelemetryRecord
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
func NewRecord() types.TelemetryRecord {
	return types.TelemetryRecord{
		SegmentsReceivedCount:  aws.Int32(0),
		SegmentsRejectedCount:  aws.Int32(0),
		SegmentsSentCount:      aws.Int32(0),
		SegmentsSpilloverCount: aws.Int32(0),
		BackendConnectionErrors: &types.BackendConnectionErrors{
			HTTPCode4XXCount:       aws.Int32(0),
			HTTPCode5XXCount:       aws.Int32(0),
			ConnectionRefusedCount: aws.Int32(0),
			OtherCount:             aws.Int32(0),
			TimeoutCount:           aws.Int32(0),
			UnknownHostCount:       aws.Int32(0),
		},
	}
}

func (tr *telemetryRecorder) HasRecording() bool {
	return tr.hasRecording.Load()
}

// Rotate the current record and swaps it out with a new record.
// Sets the timestamp and returns the old record.
func (tr *telemetryRecorder) Rotate() types.TelemetryRecord {
	snapshot := types.TelemetryRecord{
		Timestamp: aws.Time(time.Now()),
		BackendConnectionErrors: &types.BackendConnectionErrors{
			HTTPCode4XXCount:       aws.Int32(atomic.SwapInt32(tr.record.BackendConnectionErrors.HTTPCode4XXCount, 0)),
			HTTPCode5XXCount:       aws.Int32(atomic.SwapInt32(tr.record.BackendConnectionErrors.HTTPCode5XXCount, 0)),
			ConnectionRefusedCount: aws.Int32(atomic.SwapInt32(tr.record.BackendConnectionErrors.ConnectionRefusedCount, 0)),
			OtherCount:             aws.Int32(atomic.SwapInt32(tr.record.BackendConnectionErrors.OtherCount, 0)),
			TimeoutCount:           aws.Int32(atomic.SwapInt32(tr.record.BackendConnectionErrors.TimeoutCount, 0)),
			UnknownHostCount:       aws.Int32(atomic.SwapInt32(tr.record.BackendConnectionErrors.UnknownHostCount, 0)),
		},
		SegmentsReceivedCount:  aws.Int32(atomic.SwapInt32(tr.record.SegmentsReceivedCount, 0)),
		SegmentsRejectedCount:  aws.Int32(atomic.SwapInt32(tr.record.SegmentsRejectedCount, 0)),
		SegmentsSentCount:      aws.Int32(atomic.SwapInt32(tr.record.SegmentsSentCount, 0)),
		SegmentsSpilloverCount: aws.Int32(atomic.SwapInt32(tr.record.SegmentsSpilloverCount, 0)),
	}

	tr.hasRecording.Store(false)
	return snapshot
}

func (tr *telemetryRecorder) RecordSegmentsReceived(count int) {
	atomic.AddInt32(tr.record.SegmentsReceivedCount, int32(count))
	tr.hasRecording.Store(true)
}

func (tr *telemetryRecorder) RecordSegmentsSent(count int) {
	atomic.AddInt32(tr.record.SegmentsSentCount, int32(count))
	tr.hasRecording.Store(true)
}

func (tr *telemetryRecorder) RecordSegmentsSpillover(count int) {
	atomic.AddInt32(tr.record.SegmentsSpilloverCount, int32(count))
	tr.hasRecording.Store(true)
}

func (tr *telemetryRecorder) RecordSegmentsRejected(count int) {
	atomic.AddInt32(tr.record.SegmentsRejectedCount, int32(count))
	tr.hasRecording.Store(true)
}

func (tr *telemetryRecorder) RecordConnectionError(err error) {
	if err == nil {
		return
	}

	var (
		responseErr *awshttp.ResponseError
		requestErr  *smithyhttp.RequestSendError
		timeoutErr  *awshttp.ResponseTimeoutError
	)
	if ok := errors.As(err, &responseErr); ok {
		switch responseErr.HTTPStatusCode() / 100 {
		case 5:
			atomic.AddInt32(tr.record.BackendConnectionErrors.HTTPCode5XXCount, 1)
		case 4:
			atomic.AddInt32(tr.record.BackendConnectionErrors.HTTPCode4XXCount, 1)
		default:
			atomic.AddInt32(tr.record.BackendConnectionErrors.OtherCount, 1)
		}
	} else if ok := errors.As(err, &timeoutErr); ok {
		atomic.AddInt32(tr.record.BackendConnectionErrors.TimeoutCount, 1)
	} else if ok := errors.As(err, &requestErr); ok {
		atomic.AddInt32(tr.record.BackendConnectionErrors.UnknownHostCount, 1)
	} else {
		atomic.AddInt32(tr.record.BackendConnectionErrors.OtherCount, 1)
	}
	tr.hasRecording.Store(true)
}
