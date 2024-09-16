// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package jaegerencodingextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/jaegerencodingextension"

import (
	"bytes"

	"github.com/gogo/protobuf/jsonpb"
	jaegerproto "github.com/jaegertracing/jaeger/model"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/jaeger"
)

type jaegerProtobufTrace struct {
}

func (j jaegerProtobufTrace) UnmarshalTraces(buf []byte) (ptrace.Traces, error) {
	span := &jaegerproto.Span{}
	err := span.Unmarshal(buf)
	if err != nil {
		return ptrace.NewTraces(), err
	}
	return jaegerSpanToTraces(span)
}

type jaegerJSONTrace struct {
}

func (j jaegerJSONTrace) UnmarshalTraces(buf []byte) (ptrace.Traces, error) {
	span := &jaegerproto.Span{}
	err := jsonpb.Unmarshal(bytes.NewReader(buf), span)
	if err != nil {
		return ptrace.NewTraces(), err
	}
	return jaegerSpanToTraces(span)
}

func jaegerSpanToTraces(span *jaegerproto.Span) (ptrace.Traces, error) {
	batch := jaegerproto.Batch{
		Spans:   []*jaegerproto.Span{span},
		Process: span.Process,
	}
	return jaeger.ProtoToTraces([]*jaegerproto.Batch{&batch})
}
