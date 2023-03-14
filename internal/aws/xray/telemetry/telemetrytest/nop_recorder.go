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

import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray/telemetry"

func NewNopRecorder() telemetry.Recorder {
	return nopRecorderInstance
}

var nopRecorderInstance = &nopRecorder{}

var _ telemetry.Recorder = (*nopRecorder)(nil)

type nopRecorder struct {
}

func (n nopRecorder) Start() {
}

func (n nopRecorder) Stop() {
}

func (n nopRecorder) RecordSegmentsReceived(int) {
}

func (n nopRecorder) RecordSegmentsSent(int) {
}

func (n nopRecorder) RecordSegmentsSpillover(int) {
}

func (n nopRecorder) RecordSegmentsRejected(int) {
}

func (n nopRecorder) RecordConnectionError(error) {
}
