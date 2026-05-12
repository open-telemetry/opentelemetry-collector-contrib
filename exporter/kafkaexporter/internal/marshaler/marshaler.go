// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package marshaler // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter/internal/marshaler"

import (
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// TracesMarshaler marshals a ptrace.Traces into zero or more messages,
// invoking yield once per message with its key and value. Marshalers may
// pass a nil key; the exporter will set it based on partition configuration.
type TracesMarshaler interface {
	MarshalTraces(traces ptrace.Traces, yield func(key, value []byte)) error
}

// MetricsMarshaler marshals a pmetric.Metrics into zero or more messages,
// invoking yield once per message with its key and value.
type MetricsMarshaler interface {
	MarshalMetrics(metrics pmetric.Metrics, yield func(key, value []byte)) error
}

// LogsMarshaler marshals a plog.Logs into zero or more messages,
// invoking yield once per message with its key and value.
type LogsMarshaler interface {
	MarshalLogs(logs plog.Logs, yield func(key, value []byte)) error
}

// ProfilesMarshaler marshals a pprofile.Profiles into zero or more messages,
// invoking yield once per message with its key and value.
type ProfilesMarshaler interface {
	MarshalProfiles(profiles pprofile.Profiles, yield func(key, value []byte)) error
}
