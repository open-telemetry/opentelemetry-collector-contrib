// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/otelarrowreceiver/internal/metrics"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/otelarrow/admission"
	internalmetadata "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/otelarrowreceiver/internal/metadata"
)

const dataFormatProtobuf = "protobuf"

// Receiver is the type used to handle metrics from OpenTelemetry exporters.
type Receiver struct {
	pmetricotlp.UnimplementedGRPCServer
	nextConsumer     consumer.Metrics
	obsrecv          *receiverhelper.ObsReport
	boundedQueue     admission.Queue
	sizer            *pmetric.ProtoMarshaler
	logger           *zap.Logger
	telemetryBuilder *internalmetadata.TelemetryBuilder
}

// New creates a new Receiver reference.
func New(telset component.TelemetrySettings, nextConsumer consumer.Metrics, obsrecv *receiverhelper.ObsReport, bq admission.Queue) (*Receiver, error) {
	telemetryBuilder, err := internalmetadata.NewTelemetryBuilder(telset)
	if err != nil {
		return nil, err
	}
	return &Receiver{
		nextConsumer:     nextConsumer,
		obsrecv:          obsrecv,
		boundedQueue:     bq,
		sizer:            &pmetric.ProtoMarshaler{},
		logger:           telset.Logger,
		telemetryBuilder: telemetryBuilder,
	}, nil
}

// Export implements the service Export metrics func.
func (r *Receiver) Export(ctx context.Context, req pmetricotlp.ExportRequest) (pmetricotlp.ExportResponse, error) {
	md := req.Metrics()
	dataPointCount := md.DataPointCount()
	if dataPointCount == 0 {
		return pmetricotlp.NewExportResponse(), nil
	}

	sizeBytes := int64(r.sizer.MetricsSize(req.Metrics()))

	r.telemetryBuilder.OtelArrowReceiverInFlightBytes.Add(ctx, sizeBytes)
	r.telemetryBuilder.OtelArrowReceiverInFlightItems.Add(ctx, int64(dataPointCount))
	r.telemetryBuilder.OtelArrowReceiverInFlightRequests.Add(ctx, 1)

	ctx = r.obsrecv.StartMetricsOp(ctx)

	var err error
	if acqErr := r.boundedQueue.Acquire(ctx, sizeBytes); acqErr == nil {
		err = r.nextConsumer.ConsumeMetrics(ctx, md)
		// Release() is not checked, see #36074.
		_ = r.boundedQueue.Release(sizeBytes) // immediate release
	} else {
		err = acqErr
	}

	r.obsrecv.EndMetricsOp(ctx, dataFormatProtobuf, dataPointCount, err)

	r.telemetryBuilder.OtelArrowReceiverInFlightBytes.Add(ctx, -sizeBytes)
	r.telemetryBuilder.OtelArrowReceiverInFlightItems.Add(ctx, -int64(dataPointCount))
	r.telemetryBuilder.OtelArrowReceiverInFlightRequests.Add(ctx, -1)

	return pmetricotlp.NewExportResponse(), err
}

func (r *Receiver) Consumer() consumer.Metrics {
	return r.nextConsumer
}
