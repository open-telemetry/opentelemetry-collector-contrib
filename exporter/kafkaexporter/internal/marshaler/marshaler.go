// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package marshaler // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter/internal/marshaler"

import (
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// Message represents a Kafka message.
//
// Note that the topic and message headers are set by the Kafka exporter
// code, and not by the marshaler.
type Message struct {
	// Key is an optional message key.
	//
	// Marshalers may set this, but it is generally expected that the
	// Kafka producer will set this based partition_* configuration.
	Key []byte

	// Value is the message payload.
	Value []byte
}

// TracesMarshaler marshals a ptrace.Traces into one or more Messages.
type TracesMarshaler interface {
	MarshalTraces(traces ptrace.Traces) ([]Message, error)
}

// MetricsMarshaler marshals a pmetric.Metrics into one or more Messages.
type MetricsMarshaler interface {
	MarshalMetrics(metrics pmetric.Metrics) ([]Message, error)
}

// LogsMarshaler marshals a plog.Logs into one or more Messages.
type LogsMarshaler interface {
	MarshalLogs(logs plog.Logs) ([]Message, error)
}
