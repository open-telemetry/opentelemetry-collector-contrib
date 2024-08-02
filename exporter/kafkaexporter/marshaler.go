// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkaexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter"

import (
	"fmt"

	"github.com/IBM/sarama"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/zipkin/zipkinv2"
)

// TracesMarshaler marshals traces into Message array.
type TracesMarshaler interface {
	// Marshal serializes spans into sarama's ProducerMessages
	Marshal(traces ptrace.Traces, topic string) ([]*sarama.ProducerMessage, error)

	// FormatType returns format type
	FormatType() string
}

// MetricsMarshaler marshals metrics into Message array
type MetricsMarshaler interface {
	// Marshal serializes metrics into sarama's ProducerMessages
	Marshal(metrics pmetric.Metrics, topic string) ([]*sarama.ProducerMessage, error)

	// FormatType returns format type
	FormatType() string
}

// LogsMarshaler marshals logs into Message array
type LogsMarshaler interface {
	// Marshal serializes logs into sarama's ProducerMessages
	Marshal(logs plog.Logs, topic string) ([]*sarama.ProducerMessage, error)

	// FormatType returns format type
	FormatType() string
}

// tracesMarshalers returns map of supported format types with TracesMarshaler.
func tracesMarshalers() map[string]TracesMarshaler {
	otlpPb := newPdataTracesMarshaler(&ptrace.ProtoMarshaler{}, defaultFormatType)
	otlpJSON := newPdataTracesMarshaler(&ptrace.JSONMarshaler{}, "otlp_json")
	zipkinProto := newPdataTracesMarshaler(zipkinv2.NewProtobufTracesMarshaler(), "zipkin_proto")
	zipkinJSON := newPdataTracesMarshaler(zipkinv2.NewJSONTracesMarshaler(), "zipkin_json")
	jaegerProto := jaegerMarshaler{marshaler: jaegerProtoSpanMarshaler{}}
	jaegerJSON := jaegerMarshaler{marshaler: newJaegerJSONMarshaler()}
	return map[string]TracesMarshaler{
		otlpPb.FormatType():      otlpPb,
		otlpJSON.FormatType():    otlpJSON,
		zipkinProto.FormatType(): zipkinProto,
		zipkinJSON.FormatType():  zipkinJSON,
		jaegerProto.FormatType(): jaegerProto,
		jaegerJSON.FormatType():  jaegerJSON,
	}
}

// metricsMarshalers returns map of supported format types and MetricsMarshaler
func metricsMarshalers() map[string]MetricsMarshaler {
	otlpPb := newPdataMetricsMarshaler(&pmetric.ProtoMarshaler{}, defaultFormatType)
	otlpJSON := newPdataMetricsMarshaler(&pmetric.JSONMarshaler{}, "otlp_json")
	return map[string]MetricsMarshaler{
		otlpPb.FormatType():   otlpPb,
		otlpJSON.FormatType(): otlpJSON,
	}
}

// logsMarshalers returns map of supported format types and LogsMarshaler
func logsMarshalers() map[string]LogsMarshaler {
	otlpPb := newPdataLogsMarshaler(&plog.ProtoMarshaler{}, defaultFormatType)
	otlpJSON := newPdataLogsMarshaler(&plog.JSONMarshaler{}, "otlp_json")
	raw := newRawMarshaler()
	return map[string]LogsMarshaler{
		otlpPb.FormatType():   otlpPb,
		otlpJSON.FormatType(): otlpJSON,
		raw.FormatType():      raw,
	}
}

// tracesEncodingMarshaler is a wrapper around ptrace.Marshaler that implements TracesMarshaler.
type tracesEncodingMarshaler struct {
	marshaler  ptrace.Marshaler
	formatType string
}

func (t *tracesEncodingMarshaler) Marshal(traces ptrace.Traces, topic string) ([]*sarama.ProducerMessage, error) {
	var messages []*sarama.ProducerMessage
	data, err := t.marshaler.MarshalTraces(traces)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal traces: %w", err)
	}
	messages = append(messages, &sarama.ProducerMessage{
		Topic: topic,
		Value: sarama.ByteEncoder(data),
	})
	return messages, nil
}

func (t *tracesEncodingMarshaler) FormatType() string {
	return t.formatType
}

// metricsEncodingMarshaler is a wrapper around pmetric.Marshaler that implements MetricsMarshaler.
type metricsEncodingMarshaler struct {
	marshaler  pmetric.Marshaler
	formatType string
}

func (m *metricsEncodingMarshaler) Marshal(metrics pmetric.Metrics, topic string) ([]*sarama.ProducerMessage, error) {
	var messages []*sarama.ProducerMessage
	data, err := m.marshaler.MarshalMetrics(metrics)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal metrics: %w", err)
	}
	messages = append(messages, &sarama.ProducerMessage{
		Topic: topic,
		Value: sarama.ByteEncoder(data),
	})
	return messages, nil
}

func (m *metricsEncodingMarshaler) FormatType() string {
	return m.formatType
}

// logsEncodingMarshaler is a wrapper around plog.Marshaler that implements LogsMarshaler.
type logsEncodingMarshaler struct {
	marshaler  plog.Marshaler
	formatType string
}

func (l *logsEncodingMarshaler) Marshal(logs plog.Logs, topic string) ([]*sarama.ProducerMessage, error) {
	var messages []*sarama.ProducerMessage
	data, err := l.marshaler.MarshalLogs(logs)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal logs: %w", err)
	}
	messages = append(messages, &sarama.ProducerMessage{
		Topic: topic,
		Value: sarama.ByteEncoder(data),
	})
	return messages, nil
}

func (l *logsEncodingMarshaler) FormatType() string {
	return l.formatType
}
