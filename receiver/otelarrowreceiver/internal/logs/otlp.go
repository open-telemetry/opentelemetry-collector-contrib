// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logs // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/otelarrowreceiver/internal/logs"

import (
	"context"

	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/plog/plogotlp"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/otelarrow/admission"
)

const dataFormatProtobuf = "protobuf"

// Receiver is the type used to handle logs from OpenTelemetry exporters.
type Receiver struct {
	plogotlp.UnimplementedGRPCServer
	nextConsumer consumer.Logs
	obsrecv      *receiverhelper.ObsReport
	boundedQueue *admission.BoundedQueue
	sizer        *plog.ProtoMarshaler
	logger       *zap.Logger
}

// New creates a new Receiver reference.
func New(logger *zap.Logger, nextConsumer consumer.Logs, obsrecv *receiverhelper.ObsReport, bq *admission.BoundedQueue) *Receiver {
	return &Receiver{
		nextConsumer: nextConsumer,
		obsrecv:      obsrecv,
		boundedQueue: bq,
		sizer:        &plog.ProtoMarshaler{},
		logger:       logger,
	}
}

// Export implements the service Export logs func.
func (r *Receiver) Export(ctx context.Context, req plogotlp.ExportRequest) (plogotlp.ExportResponse, error) {
	ld := req.Logs()
	numSpans := ld.LogRecordCount()
	if numSpans == 0 {
		return plogotlp.NewExportResponse(), nil
	}

	ctx = r.obsrecv.StartLogsOp(ctx)

	sizeBytes := int64(r.sizer.LogsSize(req.Logs()))
	err := r.boundedQueue.Acquire(ctx, sizeBytes)
	if err != nil {
		return plogotlp.NewExportResponse(), err
	}
	defer func() {
		if releaseErr := r.boundedQueue.Release(sizeBytes); releaseErr != nil {
			r.logger.Error("Error releasing bytes from semaphore", zap.Error(releaseErr))
		}
	}()

	err = r.nextConsumer.ConsumeLogs(ctx, ld)
	r.obsrecv.EndLogsOp(ctx, dataFormatProtobuf, numSpans, err)

	return plogotlp.NewExportResponse(), err
}

func (r *Receiver) Consumer() consumer.Logs {
	return r.nextConsumer
}
