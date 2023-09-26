// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package jaeger // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/encodingextension/jaeger"

import (
	jaegerproto "github.com/jaegertracing/jaeger/model"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/jaeger"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

type jaegerProtobufTrace struct {
}

func (j jaegerProtobufTrace) UnmarshalTraces(buf []byte) (ptrace.Traces, error) {
	span := &jaegerproto.Span{}
	err := span.Unmarshal(buf)
	if err != nil {
		return ptrace.NewTraces(), err
	}
	batch := jaegerproto.Batch{
		Spans:   []*jaegerproto.Span{span},
		Process: span.Process,
	}
	return jaeger.ProtoToTraces([]*jaegerproto.Batch{&batch})
}
