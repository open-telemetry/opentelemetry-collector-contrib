// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package encoding // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"

import (
	"context"
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

// LogsUnmarshalerExtension is an extension that unmarshals logs.
type LogsUnmarshalerExtension interface {
	extension.Extension
	plog.Unmarshaler
}

// LogsStreamUnmarshaler unmarshals logs from a stream, returning one batch per call.
// UnmarshalBatch is expected to be called iteratively to read all derived plog.Logs batches from the stream.
// io.EOF is returned when there are no more batches to read.
type LogsStreamUnmarshaler interface {
	UnmarshalBatch(context.Context) (plog.Logs, error)
}

// LogsStreamUnmarshalerExtension is an extension that unmarshals logs from a stream.
type LogsStreamUnmarshalerExtension interface {
	extension.Extension
	NewStreamUnmarshaler(reader io.Reader, options ...StreamUnmarshalOption) LogsStreamUnmarshaler
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

// MetricsStreamUnmarshaler unmarshals metrics from a stream, returning one batch per call.
// UnmarshalBatch is expected to be called iteratively to read all derived pmetric.Metrics batches from the stream.
// io.EOF is returned when there are no more batches to read.
type MetricsStreamUnmarshaler interface {
	UnmarshalBatch(context.Context) (pmetric.Metrics, error)
}

// MetricsStreamUnmarshalerExtension is an extension that unmarshals metrics from a stream.
type MetricsStreamUnmarshalerExtension interface {
	extension.Extension
	NewStreamUnmarshaler(reader io.Reader, options ...StreamUnmarshalOption) MetricsStreamUnmarshaler
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
