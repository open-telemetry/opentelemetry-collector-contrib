// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkaexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter"

import (
	"github.com/IBM/sarama"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// TracesMarshaler marshals traces into Message array.
type TracesMarshaler interface {
	// Marshal serializes spans into sarama's ProducerMessages
	Marshal(traces ptrace.Traces, topic string) ([]*sarama.ProducerMessage, error)

	// Encoding returns encoding name
	Encoding() string

	// KeyData returns keyData name
	KeyData() string
}

// MetricsMarshaler marshals metrics into Message array
type MetricsMarshaler interface {
	// Marshal serializes metrics into sarama's ProducerMessages
	Marshal(metrics pmetric.Metrics, topic string) ([]*sarama.ProducerMessage, error)

	// Encoding returns encoding name
	Encoding() string
}

// LogsMarshaler marshals logs into Message array
type LogsMarshaler interface {
	// Marshal serializes logs into sarama's ProducerMessages
	Marshal(logs plog.Logs, topic string) ([]*sarama.ProducerMessage, error)

	// Encoding returns encoding name
	Encoding() string
}

// tracesMarshalers returns map of supported encodings with TracesMarshaler.
func tracesMarshalers() map[string]TracesMarshaler {
	otlpPbAndKeyNone := newPdataTracesMarshaler(&ptrace.ProtoMarshaler{}, defaultEncoding, "none")
	otlpPbAndKeyTraceId := newPdataTracesMarshaler(&ptrace.ProtoMarshaler{}, defaultEncoding, "traceID")
	otlpJsonAndKeyNone := newPdataTracesMarshaler(&ptrace.JSONMarshaler{}, "otlp_json", "none")
	otlpJsonAndKeyTraceId := newPdataTracesMarshaler(&ptrace.JSONMarshaler{}, "otlp_json", "traceID")
	jaegerProto := jaegerMarshaler{marshaler: jaegerProtoSpanMarshaler{}}
	jaegerJSON := jaegerMarshaler{marshaler: newJaegerJSONMarshaler()}
	return map[string]TracesMarshaler{
		KeyOfTracerMarshaller(otlpPbAndKeyNone):      otlpPbAndKeyNone,
		KeyOfTracerMarshaller(otlpPbAndKeyTraceId):   otlpPbAndKeyTraceId,
		KeyOfTracerMarshaller(otlpJsonAndKeyNone):    otlpJsonAndKeyNone,
		KeyOfTracerMarshaller(otlpJsonAndKeyTraceId): otlpJsonAndKeyTraceId,
		KeyOfTracerMarshaller(jaegerProto):           jaegerProto,
		KeyOfTracerMarshaller(jaegerJSON):            jaegerJSON,
	}
}

// metricsMarshalers returns map of supported encodings and MetricsMarshaler
func metricsMarshalers() map[string]MetricsMarshaler {
	otlpPb := newPdataMetricsMarshaler(&pmetric.ProtoMarshaler{}, defaultEncoding)
	otlpJSON := newPdataMetricsMarshaler(&pmetric.JSONMarshaler{}, "otlp_json")
	return map[string]MetricsMarshaler{
		otlpPb.Encoding():   otlpPb,
		otlpJSON.Encoding(): otlpJSON,
	}
}

// logsMarshalers returns map of supported encodings and LogsMarshaler
func logsMarshalers() map[string]LogsMarshaler {
	otlpPb := newPdataLogsMarshaler(&plog.ProtoMarshaler{}, defaultEncoding)
	otlpJSON := newPdataLogsMarshaler(&plog.JSONMarshaler{}, "otlp_json")
	raw := newRawMarshaler()
	return map[string]LogsMarshaler{
		otlpPb.Encoding():   otlpPb,
		otlpJSON.Encoding(): otlpJSON,
		raw.Encoding():      raw,
	}
}
