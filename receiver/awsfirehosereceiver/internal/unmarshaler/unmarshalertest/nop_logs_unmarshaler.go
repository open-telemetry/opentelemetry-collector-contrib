// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package unmarshalertest // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver/internal/unmarshaler/unmarshalertest"

import (
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver/internal/unmarshaler"
)

// NopLogsUnmarshaler is a LogsUnmarshaler that doesn't do anything
// with the inputs and just returns the logs and error passed in.
type NopLogsUnmarshaler struct {
	logs plog.Logs
	err  error
}

var _ unmarshaler.LogsUnmarshaler = (*NopLogsUnmarshaler)(nil)

// NewNopLogs provides a nop logs unmarshaler with the default
// plog.Logs and no error.
func NewNopLogs() *NopLogsUnmarshaler {
	return &NopLogsUnmarshaler{}
}

// NewWithLogs provides a nop logs unmarshaler with the passed
// in logs as the result of the Unmarshal and no error.
func NewWithLogs(logs plog.Logs) *NopLogsUnmarshaler {
	return &NopLogsUnmarshaler{logs: logs}
}

// NewErrLogs provides a nop logs unmarshaler with the passed
// in error as the Unmarshal error.
func NewErrLogs(err error) *NopLogsUnmarshaler {
	return &NopLogsUnmarshaler{err: err}
}

// Unmarshal deserializes the records into logs.
func (u *NopLogsUnmarshaler) Unmarshal([][]byte) (plog.Logs, error) {
	return u.logs, u.err
}

// Type of the serialized messages.
func (u *NopLogsUnmarshaler) Type() string {
	return typeStr
}
