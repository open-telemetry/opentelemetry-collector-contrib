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

// LogsDecoder unmarshals logs from a stream, returning one batch per call.
// DecodeLogs is expected to be called iteratively to read all derived plog.Logs batches from the stream.
// The last batch of logs should be returned with a nil error; io.EOF error should follow on the subsequent call.
type LogsDecoder interface {
	DecodeLogs(context.Context) (plog.Logs, error)
}

// LogsDecoderExtension is an extension that unmarshals logs from a stream.
type LogsDecoderExtension interface {
	extension.Extension
	NewLogsDecoder(reader io.Reader, options ...DecoderOptions) (LogsDecoder, error)
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

// MetricsDecoder unmarshals metrics from a stream, returning one batch per call.
// DecodeMetrics is expected to be called iteratively to read all derived pmetric.Metrics batches from the stream.
// The last batch of metrics should be returned with a nil error; io.EOF error should follow on the subsequent call.
type MetricsDecoder interface {
	DecodeMetrics(context.Context) (pmetric.Metrics, error)
}

// MetricsDecoderExtension is an extension that unmarshals metrics from a stream.
type MetricsDecoderExtension interface {
	extension.Extension
	NewMetricsDecoder(reader io.Reader, options ...DecoderOptions) (MetricsDecoder, error)
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

// UnmarshalOptions configures the behavior of stream unmarshaling.
type UnmarshalOptions struct {
	FlushBytes         int64
	FlushItems         int64
	StreamReaderBuffer int
}

// DecoderOptions defines the functional option for UnmarshalOptions.
type DecoderOptions func(*UnmarshalOptions)

// WithFlushBytes sets the number of bytes after stream unmarshaler should flush.
func WithFlushBytes(b int64) DecoderOptions {
	return func(o *UnmarshalOptions) {
		o.FlushBytes = b
	}
}

// WithFlushItems sets the number of items after stream unmarshaler should flush.
func WithFlushItems(i int64) DecoderOptions {
	return func(o *UnmarshalOptions) {
		o.FlushItems = i
	}
}

// WithStreamReaderBuffer sets the size of buffer that should be used by the stream reader.
func WithStreamReaderBuffer(size int) DecoderOptions {
	return func(o *UnmarshalOptions) {
		o.StreamReaderBuffer = size
	}
}
