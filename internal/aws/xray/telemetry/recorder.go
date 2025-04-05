// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package telemetry // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray/telemetry"

import (
	"errors"
	"sync/atomic"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/xray/types"
	"github.com/aws/smithy-go"
)

type Recorder interface {
	// Rotate the current record by swapping it out with a new one. Returns
	// the rotated record.
	Rotate() *types.TelemetryRecord
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
	record *types.TelemetryRecord
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
func NewRecord() *types.TelemetryRecord {
	// AWS SDK v2에서는 types 패키지 아래에 있지만, 이 함수는 v1 호환성을 위해 v1 형식을 유지
	var zero int32 = 0
	return &types.TelemetryRecord{
		SegmentsReceivedCount:  &zero,
		SegmentsRejectedCount:  &zero,
		SegmentsSentCount:      &zero,
		SegmentsSpilloverCount: &zero,
		BackendConnectionErrors: &types.BackendConnectionErrors{
			HTTPCode4XXCount:       &zero,
			HTTPCode5XXCount:       &zero,
			ConnectionRefusedCount: &zero,
			OtherCount:             &zero,
			TimeoutCount:           &zero,
			UnknownHostCount:       &zero,
		},
	}
}

func (tr *telemetryRecorder) HasRecording() bool {
	return tr.hasRecording.Load()
}

// Rotate the current record and swaps it out with a new record.
// Sets the timestamp and returns the old record.
func (tr *telemetryRecorder) Rotate() *types.TelemetryRecord {
	snapshot := NewRecord()
	snapshot.SegmentsSentCount = aws.Int32(atomic.SwapInt32(tr.record.SegmentsSentCount, 0))
	snapshot.SegmentsReceivedCount = aws.Int32(atomic.SwapInt32(tr.record.SegmentsReceivedCount, 0))
	snapshot.SegmentsRejectedCount = aws.Int32(atomic.SwapInt32(tr.record.SegmentsRejectedCount, 0))
	snapshot.SegmentsSpilloverCount = aws.Int32(atomic.SwapInt32(tr.record.SegmentsSpilloverCount, 0))
	snapshot.BackendConnectionErrors.HTTPCode4XXCount = aws.Int32(atomic.SwapInt32(tr.record.BackendConnectionErrors.HTTPCode4XXCount, 0))
	snapshot.BackendConnectionErrors.HTTPCode5XXCount = aws.Int32(atomic.SwapInt32(tr.record.BackendConnectionErrors.HTTPCode5XXCount, 0))
	snapshot.BackendConnectionErrors.TimeoutCount = aws.Int32(atomic.SwapInt32(tr.record.BackendConnectionErrors.TimeoutCount, 0))
	snapshot.BackendConnectionErrors.ConnectionRefusedCount = aws.Int32(atomic.SwapInt32(tr.record.BackendConnectionErrors.ConnectionRefusedCount, 0))
	snapshot.BackendConnectionErrors.UnknownHostCount = aws.Int32(atomic.SwapInt32(tr.record.BackendConnectionErrors.UnknownHostCount, 0))
	snapshot.BackendConnectionErrors.OtherCount = aws.Int32(atomic.SwapInt32(tr.record.BackendConnectionErrors.OtherCount, 0))
	snapshot.Timestamp = aws.Time(time.Now())
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
	
	// AWS SDK v2에서는 오류 처리가 다릅니다
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		// API 응답 코드에 따라 처리
		code := apiErr.ErrorCode()
		switch {
		case code == "RequestTimeout":
			atomic.AddInt32(tr.record.BackendConnectionErrors.TimeoutCount, 1)
		case code == "UnknownHostException":
			atomic.AddInt32(tr.record.BackendConnectionErrors.UnknownHostCount, 1)
		default:
			// HTTP 응답 코드에 따라 분류
			if len(code) >= 1 {
				switch code[0] {
				case '4':
					atomic.AddInt32(tr.record.BackendConnectionErrors.HTTPCode4XXCount, 1)
				case '5':
					atomic.AddInt32(tr.record.BackendConnectionErrors.HTTPCode5XXCount, 1)
				default:
					atomic.AddInt32(tr.record.BackendConnectionErrors.OtherCount, 1)
				}
			} else {
				atomic.AddInt32(tr.record.BackendConnectionErrors.OtherCount, 1)
			}
		}
	} else {
		// API 오류가 아닌 경우
		atomic.AddInt32(tr.record.BackendConnectionErrors.OtherCount, 1)
	}
	
	tr.hasRecording.Store(true)
}