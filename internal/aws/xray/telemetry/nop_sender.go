// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package telemetry // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray/telemetry"

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/xray/types"
)

// NewNopSender returns a Sender that drops all data.
func NewNopSender() Sender {
	return nopSenderInstance
}

var nopSenderInstance Sender = &nopSender{}

type nopSender struct{}

func (n nopSender) Rotate() types.TelemetryRecord {
	return types.TelemetryRecord{}
}

func (n nopSender) HasRecording() bool {
	return false
}

func (n nopSender) Start(_ context.Context) {
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
