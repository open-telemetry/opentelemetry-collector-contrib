// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package transport // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/statsdreceiver/internal/transport"

// NoopLogger provides a Logger that does not emit any messages
type NoopLogger struct{}

var _ Logger = (*NoopLogger)(nil)

// NewNoopLogger returns a new instance of a NoopLogger.
func NewNoopLogger() Logger {
	return &NoopLogger{}
}

func (m *NoopLogger) OnDebugf(_ string, _ ...any) {
}
