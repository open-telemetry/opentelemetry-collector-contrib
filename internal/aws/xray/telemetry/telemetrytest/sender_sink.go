// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package telemetrytest // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray/telemetry/telemetrytest"

import (
	"sync/atomic"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray/telemetry"
)

// NewSenderSink returns a Sender that acts like a sink and
// stores all calls to the record functions for testing.
func NewSenderSink() *SenderSink {
	return &SenderSink{
		Recorder:   telemetry.NewRecorder(),
		StartCount: &atomic.Int64{},
		StopCount:  &atomic.Int64{},
	}
}

var _ telemetry.Sender = (*SenderSink)(nil)

type SenderSink struct {
	telemetry.Recorder
	StartCount *atomic.Int64
	StopCount  *atomic.Int64
}

func (s SenderSink) Start() {
	s.StartCount.Add(1)
}

func (s SenderSink) Stop() {
	s.StopCount.Add(1)
}
