// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package arrow // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/otelarrowreceiver/internal/arrow"

import (
	"fmt"

	arrowpb "github.com/open-telemetry/otel-arrow/api/experimental/arrow/v1"
	"github.com/open-telemetry/otel-arrow/collector/netstats"
	arrowRecord "github.com/open-telemetry/otel-arrow/pkg/otel/arrow_record"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/extension/auth"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.opentelemetry.io/otel/trace"
)

var (
	ErrNoMetricsConsumer   = fmt.Errorf("no metrics consumer")
	ErrNoLogsConsumer      = fmt.Errorf("no logs consumer")
	ErrNoTracesConsumer    = fmt.Errorf("no traces consumer")
	ErrUnrecognizedPayload = fmt.Errorf("unrecognized OTLP payload")
)

type Consumers interface {
	Traces() consumer.Traces
	Metrics() consumer.Metrics
	Logs() consumer.Logs
}

type Receiver struct {
	Consumers

	arrowpb.UnsafeArrowTracesServiceServer
	arrowpb.UnsafeArrowLogsServiceServer
	arrowpb.UnsafeArrowMetricsServiceServer

	telemetry   component.TelemetrySettings
	tracer      trace.Tracer
	obsrecv     *receiverhelper.ObsReport
	gsettings   configgrpc.ServerConfig
	authServer  auth.Server
	newConsumer func() arrowRecord.ConsumerAPI
	netReporter netstats.Interface
}

// New creates a new Receiver reference.
func New(
	cs Consumers,
	set receiver.CreateSettings,
	obsrecv *receiverhelper.ObsReport,
	gsettings configgrpc.ServerConfig,
	authServer auth.Server,
	newConsumer func() arrowRecord.ConsumerAPI,
	netReporter netstats.Interface,
) *Receiver {
	tracer := set.TelemetrySettings.TracerProvider.Tracer("otel-arrow-receiver")
	return &Receiver{
		Consumers:   cs,
		obsrecv:     obsrecv,
		telemetry:   set.TelemetrySettings,
		tracer:      tracer,
		authServer:  authServer,
		newConsumer: newConsumer,
		gsettings:   gsettings,
		netReporter: netReporter,
	}
}

func (r *Receiver) ArrowTraces(_ arrowpb.ArrowTracesService_ArrowTracesServer) error {
	return nil
}

func (r *Receiver) ArrowLogs(_ arrowpb.ArrowLogsService_ArrowLogsServer) error {
	return nil
}

func (r *Receiver) ArrowMetrics(_ arrowpb.ArrowMetricsService_ArrowMetricsServer) error {
	return nil
}
