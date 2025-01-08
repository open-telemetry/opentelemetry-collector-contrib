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

// NewNopLogs provides a nop logs unmarshaler with the passed
// error and logs as the result of the unmarshal.
func NewNopLogs(logs plog.Logs, err error) *NopLogsUnmarshaler {
	return &NopLogsUnmarshaler{logs: logs, err: err}
}

// UnmarshalLogs deserializes the records into logs.
func (u *NopLogsUnmarshaler) UnmarshalLogs([][]byte) (plog.Logs, error) {
	return u.logs, u.err
}

// Type of the serialized messages.
func (u *NopLogsUnmarshaler) Type() string {
	return typeStr
}
