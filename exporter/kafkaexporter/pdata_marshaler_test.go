// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package kafkaexporter

import (
	"testing"

	"github.com/Shopify/sarama"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/model/otlp"
	"go.opentelemetry.io/collector/model/pdata"
)

func TestPdataTracesMarshaler(t *testing.T) {
	traceID := pdata.NewTraceID([16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})
	expectedKey := traceID.Bytes()

	td := pdata.NewTraces()
	span := td.ResourceSpans().AppendEmpty().InstrumentationLibrarySpans().AppendEmpty().Spans().AppendEmpty()
	span.SetName("foo")
	span.SetStartTimestamp(pdata.Timestamp(10))
	span.SetEndTimestamp(pdata.Timestamp(20))
	span.SetTraceID(traceID)
	span.SetSpanID(pdata.NewSpanID([8]byte{1, 2, 3, 4, 5, 6, 7, 8}))

	protoMarshaler := otlp.NewProtobufTracesMarshaler()
	protoBytes, err := protoMarshaler.MarshalTraces(td)
	require.NoError(t, err)

	jsonMarshaler := otlp.NewJSONTracesMarshaler()
	jsonBytes, err := jsonMarshaler.MarshalTraces(td)
	require.NoError(t, err)

	tests := []struct {
		marshaler TracesMarshaler
		encoding  string
		messages  []*sarama.ProducerMessage
	}{
		{
			marshaler: newPdataTracesMarshaler(protoMarshaler, "otlp_proto"),
			encoding:  "otlp_proto",
			messages: []*sarama.ProducerMessage{
				{
					Topic: "topic",
					Value: sarama.ByteEncoder(protoBytes),
					Key:   sarama.ByteEncoder(expectedKey[:]),
				},
			},
		},
		{
			marshaler: newPdataTracesMarshaler(jsonMarshaler, "otlp_json"),
			encoding:  "otlp_json",
			messages: []*sarama.ProducerMessage{
				{
					Topic: "topic",
					Value: sarama.ByteEncoder(jsonBytes),
					Key:   sarama.ByteEncoder(expectedKey[:]),
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.encoding, func(t *testing.T) {
			messages, err := test.marshaler.Marshal(td, "topic")
			require.NoError(t, err)
			assert.Equal(t, test.messages, messages)
			assert.Equal(t, test.encoding, test.marshaler.Encoding())
		})
	}
}
