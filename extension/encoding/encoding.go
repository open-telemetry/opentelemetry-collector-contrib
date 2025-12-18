// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package encoding // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"

import (
	"context"
	"errors"
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

type LogsStreamDecoderExtension interface {
	extension.Extension
	NewLogsStreamDecoder(context.Context, io.Reader, ...StreamDecoderOption) (StreamDecoder[plog.Logs], error)
}

type MetricsStreamDecoderExtension interface {
	extension.Extension
	NewMetricsStreamDecoder(context.Context, io.Reader, ...StreamDecoderOption) (StreamDecoder[pmetric.Metrics], error)
}

type TracesStreamDecoderExtension interface {
	extension.Extension
	NewTracesStreamDecoder(context.Context, io.Reader, ...StreamDecoderOption) (StreamDecoder[ptrace.Traces], error)
}

type ProfilsStreamDecoderExtension interface {
	extension.Extension
	NewProfilesStreamDecoder(context.Context, io.Reader, ...StreamDecoderOption) (StreamDecoder[pprofile.Profiles], error)
}

func GetLogsStreamDecoderExtension(ext extension.Extension) (LogsStreamDecoderExtension, error) {
	decoder, ok := ext.(LogsStreamDecoderExtension)
	if ok {
		return decoder, nil
	}
	unmarshaler, ok := ext.(LogsUnmarshalerExtension)
	if ok {
		return logsUnmarshalerStreamDecoderExtension{unmarshaler}, nil
	}
	return nil, errors.New("extension does not implement LogsStreamDecoderExtension or LogsUnmarshalerExtension")
}

func GetMetricsStreamDecoderExtension(ext extension.Extension) (MetricsStreamDecoderExtension, error) {
	decoder, ok := ext.(MetricsStreamDecoderExtension)
	if ok {
		return decoder, nil
	}
	unmarshaler, ok := ext.(MetricsUnmarshalerExtension)
	if ok {
		return metricsUnmarshalerStreamDecoderExtension{unmarshaler}, nil
	}
	return nil, errors.New("extension does not implement MetricsStreamDecoderExtension or MetricsUnmarshalerExtension")
}

type logsUnmarshalerStreamDecoderExtension struct {
	LogsUnmarshalerExtension
}

func (e logsUnmarshalerStreamDecoderExtension) NewLogsStreamDecoder(
	_ context.Context,
	r io.Reader,
	_ ...StreamDecoderOption,
) (StreamDecoder[plog.Logs], error) {
	return NewUnmarshalerStreamDecoder(r, e.LogsUnmarshalerExtension.UnmarshalLogs), nil
}

type metricsUnmarshalerStreamDecoderExtension struct {
	MetricsUnmarshalerExtension
}

func (e metricsUnmarshalerStreamDecoderExtension) NewMetricsStreamDecoder(
	_ context.Context,
	r io.Reader,
	_ ...StreamDecoderOption,
) (StreamDecoder[pmetric.Metrics], error) {
	return NewUnmarshalerStreamDecoder(r, e.MetricsUnmarshalerExtension.UnmarshalMetrics), nil
}

// TODO : Similar helper functions for Traces, and Profiles.

type unmarshalerStreamDecoder[T interface{ MoveTo(T) }] struct {
	r             io.Reader
	unmarshalFunc func([]byte) (T, error)
	offset        StreamOffset
}

// NewUnmarshalerStreamDecoder creates a StreamDecoder from an io.Reader and
// and unmarshal function. This is a helper for creating StreamDecoders from
// existing Unmarshalers.
//
// StreamDecoders returned by this function will ignore any StreamDecoderOptions.
func NewUnmarshalerStreamDecoder[T interface{ MoveTo(T) }](
	r io.Reader, unmarshalFunc func([]byte) (T, error),
) StreamDecoder[T] {
	return &unmarshalerStreamDecoder[T]{r: r, unmarshalFunc: unmarshalFunc}
}

func (d *unmarshalerStreamDecoder[T]) Offset() StreamOffset {
	return d.offset
}

func (d *unmarshalerStreamDecoder[T]) Decode(ctx context.Context, to T) error {
	data, err := io.ReadAll(d.r)
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return io.EOF
	}
	d.offset += StreamOffset(len(data))

	unmarshaled, err := d.unmarshalFunc(data)
	if err != nil {
		return err
	}
	unmarshaled.MoveTo(to)
	return nil
}
