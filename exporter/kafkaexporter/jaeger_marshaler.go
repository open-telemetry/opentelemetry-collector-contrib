// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkaexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter"

import (
	"bytes"
	"fmt"

	"github.com/IBM/sarama"
	"github.com/gogo/protobuf/jsonpb"
	jaegerproto "github.com/jaegertracing/jaeger/model"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/jaeger"
)

type jaegerMarshaler struct {
	marshaler            jaegerSpanMarshaler
	partitionedByTraceID bool
	maxMessageBytes      int
}

var _ TracesMarshaler = (*jaegerMarshaler)(nil)

func (j jaegerMarshaler) Marshal(traces ptrace.Traces, topic string) ([]*ProducerMessageChunks, error) {
	if j.maxMessageBytes <= 0 {
		return nil, fmt.Errorf("maxMessageBytes must be positive, got %d", j.maxMessageBytes)
	}

	batches := jaeger.ProtoFromTraces(traces)
	if len(batches) == 0 {
		return []*ProducerMessageChunks{}, nil
	}

	var messages []*sarama.ProducerMessage
	var messageChunks []*ProducerMessageChunks
	var packetSize int
	var errs error

	for _, batch := range batches {
		for _, span := range batch.Spans {
			span.Process = batch.Process

			bts, err := j.marshaler.marshal(span)
			if err != nil {
				errs = multierr.Append(errs, fmt.Errorf("failed to marshal span %s: %w", span.SpanID.String(), err))
				continue
			}

			key := []byte(span.TraceID.String())
			msg := &sarama.ProducerMessage{
				Topic: topic,
				Value: sarama.ByteEncoder(bts),
				Key:   sarama.ByteEncoder(key),
			}

			// Deriving version from sarama library
			// https://github.com/IBM/sarama/blob/main/async_producer.go#L454
			currentMsgSize := msg.ByteSize(2)

			// Check if message itself exceeds size limit
			if currentMsgSize > j.maxMessageBytes {
				errs = multierr.Append(errs, fmt.Errorf("span %s exceeds maximum message size: %d > %d",
					span.SpanID.String(), currentMsgSize, j.maxMessageBytes))
				continue
			}

			// Check if adding this message would exceed the chunk size limit
			if (packetSize + currentMsgSize) <= j.maxMessageBytes {
				packetSize += currentMsgSize
				messages = append(messages, msg)
			} else {
				// Current chunk is full, create new chunk
				if len(messages) > 0 {
					messageChunks = append(messageChunks, &ProducerMessageChunks{messages})
					messages = make([]*sarama.ProducerMessage, 0, len(messages)) // Preallocate with previous size
				}
				messages = append(messages, msg)
				packetSize = currentMsgSize
			}
		}
	}

	// Add final chunk if there are remaining messages
	if len(messages) > 0 {
		messageChunks = append(messageChunks, &ProducerMessageChunks{messages})
	}

	return messageChunks, errs
}

func (j jaegerMarshaler) Encoding() string {
	return j.marshaler.encoding()
}

type jaegerSpanMarshaler interface {
	marshal(span *jaegerproto.Span) ([]byte, error)
	encoding() string
}

type jaegerProtoSpanMarshaler struct{}

var _ jaegerSpanMarshaler = (*jaegerProtoSpanMarshaler)(nil)

func (p jaegerProtoSpanMarshaler) marshal(span *jaegerproto.Span) ([]byte, error) {
	return span.Marshal()
}

func (p jaegerProtoSpanMarshaler) encoding() string {
	return "jaeger_proto"
}

type jaegerJSONSpanMarshaler struct {
	pbMarshaler *jsonpb.Marshaler
}

var _ jaegerSpanMarshaler = (*jaegerJSONSpanMarshaler)(nil)

func newJaegerJSONMarshaler() *jaegerJSONSpanMarshaler {
	return &jaegerJSONSpanMarshaler{
		pbMarshaler: &jsonpb.Marshaler{},
	}
}

func (p jaegerJSONSpanMarshaler) marshal(span *jaegerproto.Span) ([]byte, error) {
	out := new(bytes.Buffer)
	err := p.pbMarshaler.Marshal(out, span)
	return out.Bytes(), err
}

func (p jaegerJSONSpanMarshaler) encoding() string {
	return "jaeger_json"
}
