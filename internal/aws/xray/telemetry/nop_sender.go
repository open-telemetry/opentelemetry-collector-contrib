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

func (nopSender) Rotate() types.TelemetryRecord {
	return types.TelemetryRecord{}
}

func (nopSender) HasRecording() bool {
	return false
}

func (nopSender) Start(context.Context) {
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
