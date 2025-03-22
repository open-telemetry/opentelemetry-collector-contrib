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

type ProducerMessageChunks struct {
	Messages []*sarama.ProducerMessage
}

// TracesMarshaler marshals traces into Message array.
type TracesMarshaler interface {
	// Marshal serializes spans into sarama's ProducerMessages
	Marshal(traces ptrace.Traces, topic string) ([]*ProducerMessageChunks, error)

	// Encoding returns encoding name
	Encoding() string
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

// creates TracesMarshaler based on the provided config
func createTracesMarshaler(config Config) (TracesMarshaler, error) {
	encoding := config.Encoding
	partitionTracesByID := config.PartitionTracesByID
	maxMessageBytes := config.Producer.MaxMessageBytes

	jaegerProto := jaegerMarshaler{marshaler: jaegerProtoSpanMarshaler{}}
	jaegerJSON := jaegerMarshaler{marshaler: newJaegerJSONMarshaler()}

	switch encoding {
	case defaultEncoding:
		return newPdataTracesMarshaler(&ptrace.ProtoMarshaler{}, defaultEncoding, partitionTracesByID, maxMessageBytes), nil
	case "otlp_json":
		return newPdataTracesMarshaler(&ptrace.JSONMarshaler{}, "otlp_json", partitionTracesByID, maxMessageBytes), nil
	case "zipkin_proto":
		return newPdataTracesMarshaler(zipkinv2.NewProtobufTracesMarshaler(), "zipkin_proto", partitionTracesByID, maxMessageBytes), nil
	case "zipkin_json":
		return newPdataTracesMarshaler(zipkinv2.NewJSONTracesMarshaler(), "zipkin_json", partitionTracesByID, maxMessageBytes), nil
	case jaegerProto.Encoding():
		return jaegerProto, nil
	case jaegerJSON.Encoding():
		return jaegerJSON, nil
	default:
		return nil, errUnrecognizedEncoding
	}
}

// creates MetricsMarshaler based on the provided config
func createMetricMarshaler(config Config) (MetricsMarshaler, error) {
	encoding := config.Encoding
	partitionMetricsByResources := config.PartitionMetricsByResourceAttributes
	switch encoding {
	case defaultEncoding:
		return newPdataMetricsMarshaler(&pmetric.ProtoMarshaler{}, defaultEncoding, partitionMetricsByResources), nil
	case "otlp_json":
		return newPdataMetricsMarshaler(&pmetric.JSONMarshaler{}, "otlp_json", partitionMetricsByResources), nil
	default:
		return nil, errUnrecognizedEncoding
	}
}

// creates LogsMarshalers based on the provided config
func createLogMarshaler(config Config) (LogsMarshaler, error) {
	encoding := config.Encoding
	partitionLogsByAttributes := config.PartitionLogsByResourceAttributes

	raw := newRawMarshaler()
	switch encoding {
	case defaultEncoding:
		return newPdataLogsMarshaler(&plog.ProtoMarshaler{}, defaultEncoding, partitionLogsByAttributes), nil
	case "otlp_json":
		return newPdataLogsMarshaler(&plog.JSONMarshaler{}, "otlp_json", partitionLogsByAttributes), nil
	case raw.Encoding():
		return raw, nil
	default:
		return nil, errUnrecognizedEncoding
	}
}

// tracesEncodingMarshaler is a wrapper around ptrace.Marshaler that implements TracesMarshaler.
type tracesEncodingMarshaler struct {
	marshaler            ptrace.Marshaler
	encoding             string
	partitionedByTraceID bool
	maxMessageBytes      int
}

func (t *tracesEncodingMarshaler) Marshal(traces ptrace.Traces, topic string) ([]*ProducerMessageChunks, error) {
	// ToDo: implement partitionedByTraceID

	messageChunks := make([]*ProducerMessageChunks, 0)
	spanCount := traces.SpanCount()

	// If the entire trace data fits within maxMessageBytes, send it as a single message
	msg := &sarama.ProducerMessage{
		Topic: topic,
	}
	data, err := t.marshaler.MarshalTraces(traces)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal traces: %w", err)
	}
	msg.Value = sarama.ByteEncoder(data)
	
	// Deriving version from sarama library
	// https://github.com/IBM/sarama/blob/main/async_producer.go#L454
	currentMsgSize := msg.ByteSize(2)

	if currentMsgSize <= t.maxMessageBytes || spanCount <= 1 {
		messageChunks = append(messageChunks, &ProducerMessageChunks{[]*sarama.ProducerMessage{msg}})
		return messageChunks, nil
	}

	// Split traces using binary search
	for i := 0; i < traces.ResourceSpans().Len(); i++ {
		rs := traces.ResourceSpans().At(i)
		for j := 0; j < rs.ScopeSpans().Len(); j++ {
			ss := rs.ScopeSpans().At(j)
			spans := ss.Spans()
			start := 0

			for start < spans.Len() {
				// Binary search to find the maximum number of spans that fit within maxMessageBytes
				left, right := 1, spans.Len()-start
				for left < right {
					mid := (left + right + 1) / 2
					chunk := ptrace.NewTraces()
					rs.CopyTo(chunk.ResourceSpans().AppendEmpty())
					newSS := chunk.ResourceSpans().At(0).ScopeSpans().AppendEmpty()
					ss.Scope().CopyTo(newSS.Scope())
					for k := 0; k < mid; k++ {
						spans.At(start + k).CopyTo(newSS.Spans().AppendEmpty())
					}

					chunkData, err := t.marshaler.MarshalTraces(chunk)
					if err != nil {
						return nil, fmt.Errorf("failed to marshal traces chunk: %w", err)
					}

					msg := &sarama.ProducerMessage{
						Topic: topic,
						Value: sarama.ByteEncoder(chunkData),
					}
					currentMsgSize := msg.ByteSize(2)

					if currentMsgSize <= t.maxMessageBytes {
						left = mid
					} else {
						right = mid - 1
					}
				}

				// Create chunk with the found number of spans
				chunk := ptrace.NewTraces()
				rs.CopyTo(chunk.ResourceSpans().AppendEmpty())
				newSS := chunk.ResourceSpans().At(0).ScopeSpans().AppendEmpty()
				ss.Scope().CopyTo(newSS.Scope())
				for k := 0; k < left; k++ {
					spans.At(start + k).CopyTo(newSS.Spans().AppendEmpty())
				}

				chunkData, err := t.marshaler.MarshalTraces(chunk)
				if err != nil {
					return nil, fmt.Errorf("failed to marshal traces chunk: %w", err)
				}

				msg := &sarama.ProducerMessage{
					Topic: topic,
					Value: sarama.ByteEncoder(chunkData),
				}
				messageChunks = append(messageChunks, &ProducerMessageChunks{[]*sarama.ProducerMessage{msg}})
				start += left
			}
		}
	}

	return messageChunks, nil
}

func (t *tracesEncodingMarshaler) Encoding() string {
	return t.encoding
}

// metricsEncodingMarshaler is a wrapper around pmetric.Marshaler that implements MetricsMarshaler.
type metricsEncodingMarshaler struct {
	marshaler pmetric.Marshaler
	encoding  string
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

func (m *metricsEncodingMarshaler) Encoding() string {
	return m.encoding
}

// logsEncodingMarshaler is a wrapper around plog.Marshaler that implements LogsMarshaler.
type logsEncodingMarshaler struct {
	marshaler plog.Marshaler
	encoding  string
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

func (l *logsEncodingMarshaler) Encoding() string {
	return l.encoding
}
