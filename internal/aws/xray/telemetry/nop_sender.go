// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package telemetry // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray/telemetry"

import "github.com/aws/aws-sdk-go/service/xray"

// NewNopSender returns a Sender that drops all data.
func NewNopSender() Sender {
	return nopSenderInstance
}

var nopSenderInstance Sender = &nopSender{}

type nopSender struct{}

func (nopSender) Rotate() *xray.TelemetryRecord {
	return nil
}

func (nopSender) HasRecording() bool {
	return false
}

func (nopSender) Start() {
}

func (nopSender) Stop() {
}

func (nopSender) RecordSegmentsReceived(int) {
}

func (nopSender) RecordSegmentsSent(int) {
}

func (nopSender) RecordSegmentsSpillover(int) {
}

func (nopSender) RecordSegmentsRejected(int) {
}

func (nopSender) RecordConnectionError(error) {
}
