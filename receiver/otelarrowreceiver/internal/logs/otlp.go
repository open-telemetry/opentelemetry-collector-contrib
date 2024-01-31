// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logs // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/otelarrowreceiver/internal/logs"

import (
	"context"

	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog/plogotlp"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
)

// Receiver is the type used to handle logs from OpenTelemetry exporters.
type Receiver struct {
	plogotlp.UnimplementedGRPCServer
	nextConsumer consumer.Logs
	obsrecv      *receiverhelper.ObsReport
}

// New creates a new Receiver reference.
func New(nextConsumer consumer.Logs, obsrecv *receiverhelper.ObsReport) *Receiver {
	return &Receiver{
		nextConsumer: nextConsumer,
		obsrecv:      obsrecv,
	}
}

// Export implements the service Export logs func.
func (r *Receiver) Export(_ context.Context, _ plogotlp.ExportRequest) (plogotlp.ExportResponse, error) {
	// TODO: Implementation.
	return plogotlp.NewExportResponse(), nil
}

func (r *Receiver) Consumer() consumer.Logs {
	return r.nextConsumer
}
