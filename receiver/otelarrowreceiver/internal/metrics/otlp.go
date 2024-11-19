// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/otelarrowreceiver/internal/metrics"

import (
	"context"

	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/otelarrow/admission2"
)

const dataFormatProtobuf = "protobuf"

// Receiver is the type used to handle metrics from OpenTelemetry exporters.
type Receiver struct {
	pmetricotlp.UnimplementedGRPCServer
	nextConsumer consumer.Metrics
	obsrecv      *receiverhelper.ObsReport
	boundedQueue admission2.Queue
	sizer        *pmetric.ProtoMarshaler
	logger       *zap.Logger
}

// New creates a new Receiver reference.
func New(logger *zap.Logger, nextConsumer consumer.Metrics, obsrecv *receiverhelper.ObsReport, bq admission2.Queue) *Receiver {
	return &Receiver{
		nextConsumer: nextConsumer,
		obsrecv:      obsrecv,
		boundedQueue: bq,
		sizer:        &pmetric.ProtoMarshaler{},
		logger:       logger,
	}
}

// Export implements the service Export metrics func.
func (r *Receiver) Export(ctx context.Context, req pmetricotlp.ExportRequest) (pmetricotlp.ExportResponse, error) {
	md := req.Metrics()
	dataPointCount := md.DataPointCount()
	if dataPointCount == 0 {
		return pmetricotlp.NewExportResponse(), nil
	}

	ctx = r.obsrecv.StartMetricsOp(ctx)

	var err error
	sizeBytes := uint64(r.sizer.MetricsSize(req.Metrics()))
	if releaser, acqErr := r.boundedQueue.Acquire(ctx, sizeBytes); acqErr == nil {
		err = r.nextConsumer.ConsumeMetrics(ctx, md)
		releaser() // immediate release
	} else {
		err = acqErr
	}

	r.obsrecv.EndMetricsOp(ctx, dataFormatProtobuf, dataPointCount, err)

	return pmetricotlp.NewExportResponse(), err
}

func (r *Receiver) Consumer() consumer.Metrics {
	return r.nextConsumer
}
