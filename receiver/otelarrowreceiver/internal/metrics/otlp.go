// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/otelarrowreceiver/internal/metrics"

import (
	"context"

	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
)

// Receiver is the type used to handle metrics from OpenTelemetry exporters.
type Receiver struct {
	pmetricotlp.UnimplementedGRPCServer
	nextConsumer consumer.Metrics
	obsrecv      *receiverhelper.ObsReport
}

// New creates a new Receiver reference.
func New(nextConsumer consumer.Metrics, obsrecv *receiverhelper.ObsReport) *Receiver {
	return &Receiver{
		nextConsumer: nextConsumer,
		obsrecv:      obsrecv,
	}
}

// Export implements the service Export metrics func.
func (r *Receiver) Export(_ context.Context, _ pmetricotlp.ExportRequest) (pmetricotlp.ExportResponse, error) {
	// TODO: Implementation.
	return pmetricotlp.NewExportResponse(), nil
}

func (r *Receiver) Consumer() consumer.Metrics {
	return r.nextConsumer
}
