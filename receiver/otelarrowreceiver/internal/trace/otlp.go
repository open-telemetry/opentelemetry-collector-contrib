// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package trace // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/otelarrowreceiver/internal/trace"

import (
	"context"

	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
)

// Receiver is the type used to handle spans from OpenTelemetry exporters.
type Receiver struct {
	ptraceotlp.UnimplementedGRPCServer
	nextConsumer consumer.Traces
	obsrecv      *receiverhelper.ObsReport
}

// New creates a new Receiver reference.
func New(nextConsumer consumer.Traces, obsrecv *receiverhelper.ObsReport) *Receiver {
	return &Receiver{
		nextConsumer: nextConsumer,
		obsrecv:      obsrecv,
	}
}

// Export implements the service Export traces func.
func (r *Receiver) Export(_ context.Context, _ ptraceotlp.ExportRequest) (ptraceotlp.ExportResponse, error) {
	// TODO: Implementation.
	return ptraceotlp.NewExportResponse(), nil
}

func (r *Receiver) Consumer() consumer.Traces {
	return r.nextConsumer
}
