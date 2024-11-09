// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package trace // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/otelarrowreceiver/internal/trace"

import (
	"context"

	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/otelarrow/admission"
)

const dataFormatProtobuf = "protobuf"

// Receiver is the type used to handle spans from OpenTelemetry exporters.
type Receiver struct {
	ptraceotlp.UnimplementedGRPCServer
	nextConsumer consumer.Traces
	obsrecv      *receiverhelper.ObsReport
	boundedQueue admission.Queue
	sizer        *ptrace.ProtoMarshaler
	logger       *zap.Logger
}

// New creates a new Receiver reference.
func New(logger *zap.Logger, nextConsumer consumer.Traces, obsrecv *receiverhelper.ObsReport, bq admission.Queue) *Receiver {
	return &Receiver{
		nextConsumer: nextConsumer,
		obsrecv:      obsrecv,
		boundedQueue: bq,
		sizer:        &ptrace.ProtoMarshaler{},
		logger:       logger,
	}
}

// Export implements the service Export traces func.
func (r *Receiver) Export(ctx context.Context, req ptraceotlp.ExportRequest) (ptraceotlp.ExportResponse, error) {
	td := req.Traces()
	numSpans := td.SpanCount()
	if numSpans == 0 {
		return ptraceotlp.NewExportResponse(), nil
	}

	ctx = r.obsrecv.StartTracesOp(ctx)

	var err error
	sizeBytes := int64(r.sizer.TracesSize(req.Traces()))
	if acqErr := r.boundedQueue.Acquire(ctx, sizeBytes); acqErr == nil {
		err = r.nextConsumer.ConsumeTraces(ctx, td)
		// Release() is not checked, see #36074.
		_ = r.boundedQueue.Release(sizeBytes) // immediate release
	} else {
		err = acqErr
	}

	r.obsrecv.EndTracesOp(ctx, dataFormatProtobuf, numSpans, err)

	return ptraceotlp.NewExportResponse(), err
}

func (r *Receiver) Consumer() consumer.Traces {
	return r.nextConsumer
}
