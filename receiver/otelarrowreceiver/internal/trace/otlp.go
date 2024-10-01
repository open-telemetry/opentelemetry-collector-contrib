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
	boundedQueue *admission.BoundedQueue
	sizer        *ptrace.ProtoMarshaler
	logger       *zap.Logger
}

// New creates a new Receiver reference.
func New(logger *zap.Logger, nextConsumer consumer.Traces, obsrecv *receiverhelper.ObsReport, bq *admission.BoundedQueue) *Receiver {
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
	// We need to ensure that it propagates the receiver name as a tag
	numSpans := td.SpanCount()
	if numSpans == 0 {
		return ptraceotlp.NewExportResponse(), nil
	}
	ctx = r.obsrecv.StartTracesOp(ctx)

	sizeBytes := int64(r.sizer.TracesSize(req.Traces()))
	err := r.boundedQueue.Acquire(ctx, sizeBytes)
	if err != nil {
		return ptraceotlp.NewExportResponse(), err
	}
	defer func() {
		if releaseErr := r.boundedQueue.Release(sizeBytes); releaseErr != nil {
			r.logger.Error("Error releasing bytes from semaphore", zap.Error(releaseErr))
		}
	}()

	err = r.nextConsumer.ConsumeTraces(ctx, td)
	r.obsrecv.EndTracesOp(ctx, dataFormatProtobuf, numSpans, err)

	return ptraceotlp.NewExportResponse(), err
}

func (r *Receiver) Consumer() consumer.Traces {
	return r.nextConsumer
}
