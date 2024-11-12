// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package trace // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/otelarrowreceiver/internal/trace"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/otelarrow/admission"
	internalmetadata "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/otelarrowreceiver/internal/metadata"
)

const dataFormatProtobuf = "protobuf"

// Receiver is the type used to handle spans from OpenTelemetry exporters.
type Receiver struct {
	ptraceotlp.UnimplementedGRPCServer
	nextConsumer     consumer.Traces
	obsrecv          *receiverhelper.ObsReport
	boundedQueue     admission.Queue
	sizer            *ptrace.ProtoMarshaler
	logger           *zap.Logger
	telemetryBuilder *internalmetadata.TelemetryBuilder
}

// New creates a new Receiver reference.
func New(telset component.TelemetrySettings, nextConsumer consumer.Traces, obsrecv *receiverhelper.ObsReport, bq admission.Queue) (*Receiver, error) {
	telemetryBuilder, err := internalmetadata.NewTelemetryBuilder(telset)
	if err != nil {
		return nil, err
	}
	return &Receiver{
		nextConsumer:     nextConsumer,
		obsrecv:          obsrecv,
		boundedQueue:     bq,
		sizer:            &ptrace.ProtoMarshaler{},
		logger:           telset.Logger,
		telemetryBuilder: telemetryBuilder,
	}, nil
}

// Export implements the service Export traces func.
func (r *Receiver) Export(ctx context.Context, req ptraceotlp.ExportRequest) (ptraceotlp.ExportResponse, error) {
	td := req.Traces()
	numSpans := td.SpanCount()
	if numSpans == 0 {
		return ptraceotlp.NewExportResponse(), nil
	}

	sizeBytes := int64(r.sizer.TracesSize(req.Traces()))

	r.telemetryBuilder.OtelArrowReceiverInFlightBytes.Add(ctx, sizeBytes)
	r.telemetryBuilder.OtelArrowReceiverInFlightItems.Add(ctx, int64(numSpans))
	r.telemetryBuilder.OtelArrowReceiverInFlightRequests.Add(ctx, 1)

	ctx = r.obsrecv.StartTracesOp(ctx)

	var err error
	if acqErr := r.boundedQueue.Acquire(ctx, sizeBytes); acqErr == nil {
		err = r.nextConsumer.ConsumeTraces(ctx, td)
		// Release() is not checked, see #36074.
		_ = r.boundedQueue.Release(sizeBytes) // immediate release
	} else {
		err = acqErr
	}

	r.telemetryBuilder.OtelArrowReceiverInFlightBytes.Add(ctx, -sizeBytes)
	r.telemetryBuilder.OtelArrowReceiverInFlightItems.Add(ctx, -int64(numSpans))
	r.telemetryBuilder.OtelArrowReceiverInFlightRequests.Add(ctx, -1)

	r.obsrecv.EndTracesOp(ctx, dataFormatProtobuf, numSpans, err)

	return ptraceotlp.NewExportResponse(), err
}

func (r *Receiver) Consumer() consumer.Traces {
	return r.nextConsumer
}
