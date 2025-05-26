// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package marshaler // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter/internal/marshaler"

import (
	"bytes"

	"github.com/gogo/protobuf/jsonpb"
	jaegerproto "github.com/jaegertracing/jaeger-idl/model/v1"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/jaeger"
)

var (
	_ TracesMarshaler = JaegerProtoSpanMarshaler{}
	_ TracesMarshaler = JaegerJSONSpanMarshaler{}
)

type JaegerProtoSpanMarshaler struct{}

type JaegerJSONSpanMarshaler struct{}

func (JaegerProtoSpanMarshaler) MarshalTraces(traces ptrace.Traces) ([]Message, error) {
	return marshalJaeger(traces, marshalJaegerSpanProto)
}

func (JaegerJSONSpanMarshaler) MarshalTraces(traces ptrace.Traces) ([]Message, error) {
	return marshalJaeger(traces, marshalJaegerSpanJSON)
}

func marshalJaeger(traces ptrace.Traces, marshal marshalJaegerSpanFunc) ([]Message, error) {
	batches := jaeger.ProtoFromTraces(traces)
	var messages []Message

	var errs error
	for _, batch := range batches {
		for _, span := range batch.Spans {
			span.Process = batch.Process
			bts, err := marshal(span)
			// continue to process spans that can be serialized
			if err != nil {
				errs = multierr.Append(errs, err)
				continue
			}
			key := []byte(span.TraceID.String())
			messages = append(messages, Message{Key: key, Value: bts})
		}
	}
	return messages, errs
}

type marshalJaegerSpanFunc func(*jaegerproto.Span) ([]byte, error)

func marshalJaegerSpanProto(span *jaegerproto.Span) ([]byte, error) {
	return span.Marshal()
}

func marshalJaegerSpanJSON(span *jaegerproto.Span) ([]byte, error) {
	var m jsonpb.Marshaler
	out := new(bytes.Buffer)
	err := m.Marshal(out, span)
	return out.Bytes(), err
}
