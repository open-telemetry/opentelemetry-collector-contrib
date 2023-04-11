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

import "github.com/aws/aws-sdk-go/service/xray"

// NewNopSender returns a Sender that drops all data.
func NewNopSender() Sender {
	return nopSenderInstance
}

var nopSenderInstance Sender = &nopSender{}

type nopSender struct {
}

func (n nopSender) Rotate() *xray.TelemetryRecord {
	return nil
}

func (n nopSender) HasRecording() bool {
	return false
}

func (n nopSender) Start() {
}

func (n nopSender) Stop() {
}

func (n nopSender) RecordSegmentsReceived(int) {
}

func (n nopSender) RecordSegmentsSent(int) {
}

func (n nopSender) RecordSegmentsSpillover(int) {
}

func (n nopSender) RecordSegmentsRejected(int) {
}

func (n nopSender) RecordConnectionError(error) {
}
