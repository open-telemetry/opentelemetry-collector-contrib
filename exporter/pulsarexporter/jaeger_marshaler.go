// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pulsarexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/pulsarexporter"

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

func (j jaegerMarshaler) Marshal(traces ptrace.Traces, _ string) ([]*pulsar.ProducerMessage, error) {
	batches, err := jaeger.ProtoFromTraces(traces)
	if err != nil {
		return nil, err
	}

	var errs error
	messages := make([]*pulsar.ProducerMessage, 0, len(batches))

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
