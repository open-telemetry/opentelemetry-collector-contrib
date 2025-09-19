// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package encoding // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"

import (
	"io"

	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// LogsMarshalerExtension is an extension that marshals logs.
type LogsMarshalerExtension interface {
	extension.Extension
	plog.Marshaler
}

// StreamingLogsUnmarshalerExtension is an extension that decodes logs from a stream.
type StreamingLogsUnmarshalerExtension interface {
	extension.Extension
	NewLogsDecoder(io.Reader) (LogsDecoder, error)
}

type LogsDecoder interface {
	// DecodeLogs decodes logs and appends them to the given plog.Logs.
	//
	// DecodeLogs returns io.EOF if there are no more logs to be decoded.
	DecodeLogs(to plog.Logs) error
}

// LogsUnmarshalerExtension is an extension that unmarshals logs.
type LogsUnmarshalerExtension interface {
	extension.Extension
	plog.Unmarshaler
}

// MetricsMarshalerExtension is an extension that marshals metrics.
type MetricsMarshalerExtension interface {
	extension.Extension
	pmetric.Marshaler
}

// MetricsUnmarshalerExtension is an extension that unmarshals metrics.
type MetricsUnmarshalerExtension interface {
	extension.Extension
	pmetric.Unmarshaler
}

// TracesMarshalerExtension is an extension that marshals traces.
type TracesMarshalerExtension interface {
	extension.Extension
	ptrace.Marshaler
}

// TracesUnmarshalerExtension is an extension that unmarshals traces.
type TracesUnmarshalerExtension interface {
	extension.Extension
	ptrace.Unmarshaler
}

// ProfilesMarshalerExtension is an extension that marshals profiles.
type ProfilesMarshalerExtension interface {
	extension.Extension
	pprofile.Marshaler
}

// ProfilesUnmarshalerExtension is an extension that unmarshals Profiles.
type ProfilesUnmarshalerExtension interface {
	extension.Extension
	pprofile.Unmarshaler
}
