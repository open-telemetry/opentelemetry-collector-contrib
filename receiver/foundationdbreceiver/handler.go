// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package foundationdbreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/foundationdbreceiver"

import (
	"context"
	"io"

	"github.com/vmihailenco/msgpack/v5"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.uber.org/zap"
)

type fdbTraceHandler interface {
	Handle(data []byte) error
}

type openTracingHandler struct {
	consumer consumer.Traces
	obsrecv  *receiverhelper.ObsReport
	logger   *zap.Logger
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
	obsrecv  *receiverhelper.ObsReport
	logger   *zap.Logger
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
