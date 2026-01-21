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

type LogsStreamer func(context.Context) (plog.Logs, error)

// LogsStreamUnmarshalExtension is an extension that unmarshals logs from a stream.
type LogsStreamUnmarshalExtension interface {
	extension.Extension
	GetStreamUnmarshaler(reader io.Reader, options ...StreamUnmarshalOption) LogsStreamer
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

// StreamUnmarshalOptions configures the behavior of stream unmarshaling.
type StreamUnmarshalOptions struct {
	FlushBytes int64
	FlushItems int64
}

// StreamUnmarshalOption defines the functional option for StreamUnmarshalOptions.
type StreamUnmarshalOption func(*StreamUnmarshalOptions)

// WithFlushBytes sets the number of bytes after stream unmarshaler should flush.
func WithFlushBytes(b int64) StreamUnmarshalOption {
	return func(o *StreamUnmarshalOptions) {
		o.FlushBytes = b
	}
}

// WithFlushItems sets the number of items after stream unmarshaler should flush.
func WithFlushItems(i int64) StreamUnmarshalOption {
	return func(o *StreamUnmarshalOptions) {
		o.FlushItems = i
	}
}

// StreamTerminalHelper is a helper to determine when to flush based on configured options.
type StreamTerminalHelper struct {
	options      StreamUnmarshalOptions
	currentBytes int64
	currentItems int64
}

func NewStreamTerminalHelper(opts ...StreamUnmarshalOption) *StreamTerminalHelper {
	options := StreamUnmarshalOptions{}
	for _, o := range opts {
		o(&options)
	}
	return &StreamTerminalHelper{
		options: options,
	}
}

func (sh *StreamTerminalHelper) IncrementBytes(n int64) {
	sh.currentBytes += n
}

func (sh *StreamTerminalHelper) IncrementItems(n int64) {
	sh.currentItems += n
}

func (sh *StreamTerminalHelper) ShouldFlush() bool {
	if sh.options.FlushBytes > 0 && sh.currentBytes >= sh.options.FlushBytes {
		return true
	}
	if sh.options.FlushItems > 0 && sh.currentItems >= sh.options.FlushItems {
		return true
	}
	return false
}

func (sh *StreamTerminalHelper) Reset() {
	sh.currentBytes = 0
	sh.currentItems = 0
}
