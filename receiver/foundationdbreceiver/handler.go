// Copyright 2022 OpenTelemetry Authors
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
// See the License for the specific language

package foundationdbreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/foundationdbreceiver"

import (
	"context"
	"io"

	"github.com/vmihailenco/msgpack/v5"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/obsreport"
)

type fdbTraceHandler interface {
	Handle(data []byte) error
}

type openTracingHandler struct {
	consumer consumer.Traces
	obsrecv  *obsreport.Receiver
}

func (h *openTracingHandler) Handle(data []byte) error {
	ctx := context.Background()
	h.obsrecv.StartTracesOp(ctx)
	traces := ptrace.NewTraces()
	var trace OpenTracing
	err := msgpack.Unmarshal(data, &trace)
	if err != nil {
		if err != io.EOF {
			h.obsrecv.EndTracesOp(ctx, OPENTRACING, 0, err)
			return err
		}
	}
	span := traces.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	trace.getSpan(&span)
	err = h.consumer.ConsumeTraces(context.Background(), traces)
	h.obsrecv.EndTracesOp(ctx, OPENTRACING, traces.SpanCount(), err)
	return err
}

type openTelemetryHandler struct {
	consumer consumer.Traces
	obsrecv  *obsreport.Receiver
}

func (h *openTelemetryHandler) Handle(data []byte) error {
	ctx := context.Background()
	traces := ptrace.NewTraces()
	var trace Trace
	err := msgpack.Unmarshal(data, &trace)
	if err != nil {
		if err != io.EOF {
			h.obsrecv.EndTracesOp(ctx, OPENTELEMETRY, 0, err)
			return err
		}
	}
	span := traces.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	trace.getSpan(&span)
	err = h.consumer.ConsumeTraces(context.Background(), traces)
	h.obsrecv.EndTracesOp(ctx, OPENTELEMETRY, traces.SpanCount(), err)
	return err
}
