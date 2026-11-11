// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package windows // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/windows"

import "context"

// WindowsInputTelemetry is implemented by the receiver and injected into the
// operator so that self-monitoring metrics can be recorded without creating a
// reverse dependency from pkg/stanza into the receiver.
type WindowsInputTelemetry interface {
	RecordEventSize(ctx context.Context, channel string, sizeBytes int)
	RecordChannelSize(ctx context.Context, channel string, size int64)
	RecordMissedEvents(ctx context.Context, channel string, count int64)
	RecordBatchSize(ctx context.Context, channel string, count int64)
}
