// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelarrowreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/otelarrowreceiver"

import (
	"context"
	"errors"
	"sync"

	arrowpb "github.com/open-telemetry/otel-arrow/api/experimental/arrow/v1"
	arrowRecord "github.com/open-telemetry/otel-arrow/pkg/otel/arrow_record"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/extension/auth"
	"go.opentelemetry.io/collector/pdata/plog/plogotlp"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/otelarrow/admission"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/otelarrow/compression/zstd"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/otelarrow/netstats"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/otelarrowreceiver/internal/arrow"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/otelarrowreceiver/internal/logs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/otelarrowreceiver/internal/metrics"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/otelarrowreceiver/internal/trace"
)

// otelArrowReceiver is the type that exposes Trace and Metrics reception.
type otelArrowReceiver struct {
	cfg        *Config
	serverGRPC *grpc.Server

	tracesReceiver  *trace.Receiver
	metricsReceiver *metrics.Receiver
	logsReceiver    *logs.Receiver
	arrowReceiver   *arrow.Receiver
	shutdownWG      sync.WaitGroup

	obsrepGRPC  *receiverhelper.ObsReport
	netReporter *netstats.NetworkReporter

	settings receiver.Settings
}

// newOTelArrowReceiver just creates the OpenTelemetry receiver services. It is the caller's
// responsibility to invoke the respective Start*Reception methods as well
// as the various Stop*Reception methods to end it.
func newOTelArrowReceiver(cfg *Config, set receiver.Settings) (*otelArrowReceiver, error) {
	netReporter, err := netstats.NewReceiverNetworkReporter(set)
	if err != nil {
		return nil, err
	}
	r := &otelArrowReceiver{
		cfg:         cfg,
		settings:    set,
		netReporter: netReporter,
	}
	if err = zstd.SetDecoderConfig(cfg.Arrow.Zstd); err != nil {
		return nil, err
	}

	r.obsrepGRPC, err = receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
		ReceiverID:             set.ID,
		Transport:              "grpc",
		ReceiverCreateSettings: set,
	})
	if err != nil {
		return nil, err
	}

	return r, nil
}

func (r *otelArrowReceiver) startGRPCServer(cfg configgrpc.ServerConfig, _ component.Host) error {
	r.settings.Logger.Info("Starting GRPC server", zap.String("endpoint", cfg.NetAddr.Endpoint))

	gln, err := cfg.NetAddr.Listen(context.Background())
	if err != nil {
		return err
	}
	r.shutdownWG.Add(1)
	go func() {
		defer r.shutdownWG.Done()

		if errGrpc := r.serverGRPC.Serve(gln); errGrpc != nil && !errors.Is(errGrpc, grpc.ErrServerStopped) {
			r.settings.ReportStatus(component.NewFatalErrorEvent(errGrpc))
		}
	}()
	return nil
}

func (r *otelArrowReceiver) startProtocolServers(ctx context.Context, host component.Host) error {
	var err error
	var serverOpts []grpc.ServerOption

	if r.netReporter != nil {
		serverOpts = append(serverOpts, grpc.StatsHandler(r.netReporter.Handler()))
	}
	r.serverGRPC, err = r.cfg.GRPC.ToServer(ctx, host, r.settings.TelemetrySettings, serverOpts...)
	if err != nil {
		return err
	}

	var authServer auth.Server
	if r.cfg.GRPC.Auth != nil {
		authServer, err = r.cfg.GRPC.Auth.GetServerAuthenticator(ctx, host.GetExtensions())
		if err != nil {
			return err
		}
	}
	bq := admission.NewBoundedQueue(int64(r.cfg.Arrow.AdmissionLimitMiB<<20), r.cfg.Arrow.WaiterLimit)

	r.arrowReceiver, err = arrow.New(arrow.Consumers(r), r.settings, r.obsrepGRPC, r.cfg.GRPC, authServer, func() arrowRecord.ConsumerAPI {
		var opts []arrowRecord.Option
		if r.cfg.Arrow.MemoryLimitMiB != 0 {
			// in which case the default is selected in the arrowRecord package.
			opts = append(opts, arrowRecord.WithMemoryLimit(r.cfg.Arrow.MemoryLimitMiB<<20))
		}
		if r.settings.TelemetrySettings.MeterProvider != nil {
			opts = append(opts, arrowRecord.WithMeterProvider(r.settings.TelemetrySettings.MeterProvider, r.settings.TelemetrySettings.MetricsLevel))
		}
		return arrowRecord.NewConsumer(opts...)
	}, bq, r.netReporter)

	if err != nil {
		return err
	}

	if r.tracesReceiver != nil {
		ptraceotlp.RegisterGRPCServer(r.serverGRPC, r.tracesReceiver)

		arrowpb.RegisterArrowTracesServiceServer(r.serverGRPC, r.arrowReceiver)
	}

	if r.metricsReceiver != nil {
		pmetricotlp.RegisterGRPCServer(r.serverGRPC, r.metricsReceiver)

		arrowpb.RegisterArrowMetricsServiceServer(r.serverGRPC, r.arrowReceiver)
	}

	if r.logsReceiver != nil {
		plogotlp.RegisterGRPCServer(r.serverGRPC, r.logsReceiver)

		arrowpb.RegisterArrowLogsServiceServer(r.serverGRPC, r.arrowReceiver)
	}

	err = r.startGRPCServer(r.cfg.GRPC, host)
	if err != nil {
		return err
	}

	return err
}

// Start runs the trace receiver on the gRPC server. Currently
// it also enables the metrics receiver too.
func (r *otelArrowReceiver) Start(ctx context.Context, host component.Host) error {
	return r.startProtocolServers(ctx, host)
}

// Shutdown is a method to turn off receiving.
func (r *otelArrowReceiver) Shutdown(_ context.Context) error {
	var err error

	if r.serverGRPC != nil {
		r.serverGRPC.GracefulStop()
	}

	r.shutdownWG.Wait()
	return err
}

func (r *otelArrowReceiver) registerTraceConsumer(tc consumer.Traces) {
	r.tracesReceiver = trace.New(tc, r.obsrepGRPC)
}

func (r *otelArrowReceiver) registerMetricsConsumer(mc consumer.Metrics) {
	r.metricsReceiver = metrics.New(mc, r.obsrepGRPC)
}

func (r *otelArrowReceiver) registerLogsConsumer(lc consumer.Logs) {
	r.logsReceiver = logs.New(lc, r.obsrepGRPC)
}

var _ arrow.Consumers = &otelArrowReceiver{}

func (r *otelArrowReceiver) Traces() consumer.Traces {
	if r.tracesReceiver == nil {
		return nil
	}
	return r.tracesReceiver.Consumer()
}

func (r *otelArrowReceiver) Metrics() consumer.Metrics {
	if r.metricsReceiver == nil {
		return nil
	}
	return r.metricsReceiver.Consumer()
}

func (r *otelArrowReceiver) Logs() consumer.Logs {
	if r.logsReceiver == nil {
		return nil
	}
	return r.logsReceiver.Consumer()
}
