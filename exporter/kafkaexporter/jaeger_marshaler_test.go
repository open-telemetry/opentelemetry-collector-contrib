// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkaexporter

import (
	"bytes"
	"testing"

	"github.com/IBM/sarama"
	"github.com/gogo/protobuf/jsonpb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/jaeger"
)

func TestJaegerMarshaler(t *testing.T) {
	td := ptrace.NewTraces()
	span := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.SetName("foo")
	span.SetStartTimestamp(pcommon.Timestamp(10))
	span.SetEndTimestamp(pcommon.Timestamp(20))
	span.SetTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	span.SetSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8})
	batches := jaeger.ProtoFromTraces(td)

	batches[0].Spans[0].Process = batches[0].Process
	jaegerProtoBytes, err := batches[0].Spans[0].Marshal()
	messageKey := []byte(batches[0].Spans[0].TraceID.String())
	require.NoError(t, err)
	require.NotNil(t, jaegerProtoBytes)

	jsonMarshaler := &jsonpb.Marshaler{}
	jsonByteBuffer := new(bytes.Buffer)
	require.NoError(t, jsonMarshaler.Marshal(jsonByteBuffer, batches[0].Spans[0]))

	tests := []struct {
		name        string
		unmarshaler jaegerMarshaler
		encoding    string
		messages    []*ProducerMessageChunks
	}{
		{
			name: "proto_marshaler",
			unmarshaler: jaegerMarshaler{
				marshaler:       jaegerProtoSpanMarshaler{},
				maxMessageBytes: defaultProducerMaxMessageBytes,
			},
			encoding: "jaeger_proto",
			messages: []*ProducerMessageChunks{
				{
					Messages: []*sarama.ProducerMessage{
						{
							Topic: "topic",
							Value: sarama.ByteEncoder(jaegerProtoBytes),
							Key:   sarama.ByteEncoder(messageKey),
						},
					},
				},
			},
		},
		{
			name: "json_marshaler",
			unmarshaler: jaegerMarshaler{
				marshaler: jaegerJSONSpanMarshaler{
					pbMarshaler: &jsonpb.Marshaler{},
				},
				maxMessageBytes: defaultProducerMaxMessageBytes,
			},
			encoding: "jaeger_json",
			messages: []*ProducerMessageChunks{
				{
					Messages: []*sarama.ProducerMessage{
						{
							Topic: "topic",
							Value: sarama.ByteEncoder(jsonByteBuffer.Bytes()),
							Key:   sarama.ByteEncoder(messageKey),
						},
					},
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			msg, err := test.unmarshaler.Marshal(td, "topic")
			require.NoError(t, err)
			assert.Equal(t, test.messages, msg)
			assert.Equal(t, test.encoding, test.unmarshaler.Encoding())
		})
	}
}
