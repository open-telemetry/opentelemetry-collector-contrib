// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkaexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter"

import (
	"bytes"

	"github.com/IBM/sarama"
	"github.com/gogo/protobuf/jsonpb"
	jaegerproto "github.com/jaegertracing/jaeger/model"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/jaeger"
)

type jaegerMarshaler struct {
	marshaler jaegerSpanMarshaler
}

var _ TracesMarshaler = (*jaegerMarshaler)(nil)

func (j jaegerMarshaler) Marshal(traces ptrace.Traces, topic string, config *Config) ([][]*sarama.ProducerMessage, error) {
	batches, err := jaeger.ProtoFromTraces(traces)
	if err != nil {
		return nil, err
	}
	messagesSlice := [][]*sarama.ProducerMessage{make([]*sarama.ProducerMessage, 0)}

	var errs error
	var messagesByteSize, messageSliceIdx int
	err = setKafkaProtoVersion(config)
	if err != nil {
		return nil, err
	}

	for _, batch := range batches {
		for _, span := range batch.Spans {
			span.Process = batch.Process
			bts, err := j.marshaler.marshal(span)
			// continue to process spans that can be serialized
			if err != nil {
				errs = multierr.Append(errs, err)
				continue
			}
			key := []byte(span.TraceID.String())
			message := &sarama.ProducerMessage{
				Topic: topic,
				Value: sarama.ByteEncoder(bts),
				Key:   sarama.ByteEncoder(key),
			}
			if message.ByteSize(config.Producer.protoVersion) > config.Producer.MaxMessageBytes {
				return nil, errSingleJaegerSpanMessageSizeOverMaxMsgByte
			}
			messagesByteSize += message.ByteSize(config.Producer.protoVersion)
			if messagesByteSize > config.Producer.MaxMessageBytes {
				messageSliceIdx += 1
				messagesSlice = append(messagesSlice, []*sarama.ProducerMessage{})
			}
			messagesSlice[messageSliceIdx] = append(messagesSlice[messageSliceIdx], message)
		}
	}
	return messagesSlice, errs
}

func (j jaegerMarshaler) Encoding() string {
	return j.marshaler.encoding()
}

type jaegerSpanMarshaler interface {
	marshal(span *jaegerproto.Span) ([]byte, error)
	encoding() string
}

type jaegerProtoSpanMarshaler struct {
}

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
