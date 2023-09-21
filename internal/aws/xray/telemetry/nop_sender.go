// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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
