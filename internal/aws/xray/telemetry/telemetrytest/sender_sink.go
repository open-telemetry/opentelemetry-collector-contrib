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
