// Copyright 2020 The OpenTelemetry Authors
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

package pulsarexporter

import (
	"bytes"

	"github.com/apache/pulsar-client-go/pulsar"
	"github.com/gogo/protobuf/jsonpb"
	jaegerproto "github.com/jaegertracing/jaeger/model"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/jaeger"
)

type jaegerMarshaler struct {
	marshaler jaegerBatchMarshaler
}

var _ TracesMarshaler = (*jaegerMarshaler)(nil)

func (j jaegerMarshaler) Marshal(traces ptrace.Traces, topic string) ([]*pulsar.ProducerMessage, error) {
	batches, err := jaeger.ProtoFromTraces(traces)
	if err != nil {
		return nil, err
	}

	var errs error
	var messages []*pulsar.ProducerMessage

	for _, batch := range batches {
		bts, err := j.marshaler.marshal(batch)
		if err != nil {
			errs = multierr.Append(errs, err)
			continue
		}

		messages = append(messages, &pulsar.ProducerMessage{
			Payload: bts,
		})
	}

	return messages, errs
}

func (j jaegerMarshaler) Encoding() string {
	return j.marshaler.encoding()
}

type jaegerBatchMarshaler interface {
	marshal(batch *jaegerproto.Batch) ([]byte, error)
	encoding() string
}

type jaegerProtoBatchMarshaler struct {
}

var _ jaegerBatchMarshaler = (*jaegerProtoBatchMarshaler)(nil)

func (p jaegerProtoBatchMarshaler) marshal(batch *jaegerproto.Batch) ([]byte, error) {
	return batch.Marshal()
}

func (p jaegerProtoBatchMarshaler) encoding() string {
	return "jaeger_proto"
}

type jaegerJSONBatchMarshaler struct {
	pbMarshaler *jsonpb.Marshaler
}

var _ jaegerBatchMarshaler = (*jaegerJSONBatchMarshaler)(nil)

func newJaegerJSONMarshaler() *jaegerJSONBatchMarshaler {
	return &jaegerJSONBatchMarshaler{
		pbMarshaler: &jsonpb.Marshaler{},
	}
}

func (p jaegerJSONBatchMarshaler) marshal(batch *jaegerproto.Batch) ([]byte, error) {
	out := new(bytes.Buffer)
	err := p.pbMarshaler.Marshal(out, batch)
	return out.Bytes(), err
}

func (p jaegerJSONBatchMarshaler) encoding() string {
	return "jaeger_json"
}
