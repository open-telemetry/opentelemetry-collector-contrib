// Copyright  OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package kafkaexporter

import (
	"github.com/Shopify/sarama"
	"go.opentelemetry.io/collector/model/pdata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/batchpersignal"
)

type keyedPdataTracesMarshaler struct {
	marshaler pdata.TracesMarshaler
	encoding  string
}

func (p keyedPdataTracesMarshaler) Marshal(td pdata.Traces, topic string) ([]*sarama.ProducerMessage, error) {
	var messages []*sarama.ProducerMessage
	for _, batch := range batchpersignal.SplitTraces(td) {
		bts, err := p.marshaler.MarshalTraces(batch)
		if err != nil {
			return nil, err
		}
		// every batch should have at least one span
		traceID := batch.ResourceSpans().At(0).InstrumentationLibrarySpans().At(0).Spans().At(0).TraceID().Bytes()
		messages = append(messages, &sarama.ProducerMessage{
			Topic: topic,
			Key:   sarama.ByteEncoder(traceID[:]),
			Value: sarama.ByteEncoder(bts),
		})
	}
	return messages, nil
}

func (p keyedPdataTracesMarshaler) Encoding() string {
	return p.encoding
}

func newKeyedPdataTracesMarshaler(marshaler pdata.TracesMarshaler, encoding string) TracesMarshaler {
	return keyedPdataTracesMarshaler{
		marshaler: marshaler,
		encoding:  encoding,
	}
}
