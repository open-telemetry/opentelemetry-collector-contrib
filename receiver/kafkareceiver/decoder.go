// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkareceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkareceiver"

import (
	"bytes"
	"context"
	"io"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/receiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkareceiver/internal/metadata"
)

type messageDecoder[T any] func(context.Context, []byte) (T, error)

// Preserve the byte-only path for existing extensions implementing both APIs:
// their streaming methods can have different framing or compression requirements.
// The feature gate allows explicitly opting into that migration.
func loadStreamingDecoder[Factory, Unmarshaler any](host component.Host, name, signal string) (Factory, bool) {
	factory, err := loadEncodingExtension[Factory](host, name, signal)
	if err != nil {
		return factory, false
	}
	if metadata.ReceiverKafkaPreferStreamingEncodingFeatureGate.IsEnabled() {
		return factory, true
	}
	_, err = loadEncodingExtension[Unmarshaler](host, name, signal)
	return factory, err != nil
}

func newLogsDecoder(name string, set receiver.Settings, host component.Host) (messageDecoder[plog.Logs], error) {
	if factory, ok := loadStreamingDecoder[encoding.LogsDecoderFactory, plog.Unmarshaler](host, name, "logs"); ok {
		return func(ctx context.Context, data []byte) (plog.Logs, error) {
			decoder, err := factory.NewLogsDecoder(bytes.NewReader(data), encoding.WithContext(ctx))
			if err != nil {
				return plog.Logs{}, err
			}
			return decodeBatches(decoder.DecodeLogs, plog.NewLogs, func(src, dst plog.Logs) error {
				src.ResourceLogs().MoveAndAppendTo(dst.ResourceLogs())
				return nil
			})
		}, nil
	}
	unmarshaler, err := newLogsUnmarshaler(name, set, host)
	if err != nil {
		return nil, err
	}
	return func(_ context.Context, data []byte) (plog.Logs, error) {
		return unmarshaler.UnmarshalLogs(data)
	}, nil
}

func newMetricsDecoder(name string, set receiver.Settings, host component.Host) (messageDecoder[pmetric.Metrics], error) {
	if factory, ok := loadStreamingDecoder[encoding.MetricsDecoderFactory, pmetric.Unmarshaler](host, name, "metrics"); ok {
		return func(ctx context.Context, data []byte) (pmetric.Metrics, error) {
			decoder, err := factory.NewMetricsDecoder(bytes.NewReader(data), encoding.WithContext(ctx))
			if err != nil {
				return pmetric.Metrics{}, err
			}
			return decodeBatches(decoder.DecodeMetrics, pmetric.NewMetrics, func(src, dst pmetric.Metrics) error {
				src.ResourceMetrics().MoveAndAppendTo(dst.ResourceMetrics())
				return nil
			})
		}, nil
	}
	unmarshaler, err := newMetricsUnmarshaler(name, set, host)
	if err != nil {
		return nil, err
	}
	return func(_ context.Context, data []byte) (pmetric.Metrics, error) {
		return unmarshaler.UnmarshalMetrics(data)
	}, nil
}

func newTracesDecoder(name string, set receiver.Settings, host component.Host) (messageDecoder[ptrace.Traces], error) {
	if factory, ok := loadStreamingDecoder[encoding.TracesDecoderFactory, ptrace.Unmarshaler](host, name, "traces"); ok {
		return func(ctx context.Context, data []byte) (ptrace.Traces, error) {
			decoder, err := factory.NewTracesDecoder(bytes.NewReader(data), encoding.WithContext(ctx))
			if err != nil {
				return ptrace.Traces{}, err
			}
			return decodeBatches(decoder.DecodeTraces, ptrace.NewTraces, func(src, dst ptrace.Traces) error {
				src.ResourceSpans().MoveAndAppendTo(dst.ResourceSpans())
				return nil
			})
		}, nil
	}
	unmarshaler, err := newTracesUnmarshaler(name, set, host)
	if err != nil {
		return nil, err
	}
	return func(_ context.Context, data []byte) (ptrace.Traces, error) {
		return unmarshaler.UnmarshalTraces(data)
	}, nil
}

func newProfilesDecoder(name string, set receiver.Settings, host component.Host) (messageDecoder[pprofile.Profiles], error) {
	if factory, ok := loadStreamingDecoder[encoding.ProfilesDecoderFactory, pprofile.Unmarshaler](host, name, "profiles"); ok {
		return func(ctx context.Context, data []byte) (pprofile.Profiles, error) {
			decoder, err := factory.NewProfilesDecoder(bytes.NewReader(data), encoding.WithContext(ctx))
			if err != nil {
				return pprofile.Profiles{}, err
			}
			return decodeBatches(decoder.DecodeProfiles, pprofile.NewProfiles, pprofile.Profiles.MergeTo)
		}, nil
	}
	unmarshaler, err := newProfilesUnmarshaler(name, set, host)
	if err != nil {
		return nil, err
	}
	return func(_ context.Context, data []byte) (pprofile.Profiles, error) {
		return unmarshaler.UnmarshalProfiles(data)
	}, nil
}

// decodeBatches collects a single Kafka message before forwarding it, so a
// decoding failure cannot send only part of a message to the next consumer.
// Keep the first batch intact to avoid copying or reindexing single-batch data.
func decodeBatches[T any](decode func() (T, error), empty func() T, merge func(T, T) error) (T, error) {
	data, err := decode()
	if err == io.EOF { //nolint:errorlint // Wrapped EOF can be a parse error, not stream completion.
		return empty(), nil
	}
	if err != nil {
		return data, err
	}
	for {
		batch, err := decode()
		// A wrapped EOF can be a parse error rather than stream completion.
		if err == io.EOF { //nolint:errorlint // Only the terminal EOF sentinel ends the stream.
			return data, nil
		}
		if err != nil {
			var zero T
			return zero, err
		}
		if err := merge(batch, data); err != nil {
			var zero T
			return zero, err
		}
	}
}
