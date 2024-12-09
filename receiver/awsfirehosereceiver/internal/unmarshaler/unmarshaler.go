// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package unmarshaler // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver/internal/unmarshaler"

import (
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

// MetricsUnmarshaler deserializes the message body
type MetricsUnmarshaler interface {
	// UnmarshalMetrics deserializes the records into metrics.
	UnmarshalMetrics(contentType string, records [][]byte) (pmetric.Metrics, error)

	// Type of the serialized messages.
	Type() string
}

// LogsUnmarshaler deserializes the message body
type LogsUnmarshaler interface {
	// UnmarshalLogs deserializes the records into logs.
	UnmarshalLogs(contentType string, records [][]byte) (plog.Logs, error)

	// Type of the serialized messages.
	Type() string
}

// Unmarshaler deserializes the message body
type Unmarshaler interface {
	// UnmarshalMetrics deserializes the records into metrics.
	UnmarshalMetrics(contentType string, records [][]byte) (pmetric.Metrics, error)

	// UnmarshalLogs deserializes the records into logs.
	UnmarshalLogs(contentType string, records [][]byte) (plog.Logs, error)

	// Type of the serialized messages.
	Type() string
}
