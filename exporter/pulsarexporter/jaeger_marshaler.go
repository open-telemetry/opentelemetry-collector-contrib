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
	marshaler jaegerSpanMarshaler
}

var _ TracesMarshaler = (*jaegerMarshaler)(nil)

func (j jaegerMarshaler) Marshal(traces ptrace.Traces, topic string) ([]*pulsar.ProducerMessage, error) {
	batches, err := jaeger.ProtoFromTraces(traces)
	if err != nil {
		return nil, err
	}
	var messages []*pulsar.ProducerMessage

	var errs error
	for _, batch := range batches {
		for _, span := range batch.Spans {
			span.Process = batch.Process
			bts, err := j.marshaler.marshal(span)
			// continue to process spans that can be serialized
			if err != nil {
				errs = multierr.Append(errs, err)
				continue
			}

			messages = append(messages, &pulsar.ProducerMessage{
				Payload: bts,
				Key:     span.TraceID.String(),
			})
		}
	}
	return messages, errs
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
