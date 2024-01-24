// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelarrowreceiver // import "github.com/open-telemetry/otel-arrow/collector/receiver/otelarrowreceiver"

import (
	"context"
	"errors"
	"sync"

	arrowpb "github.com/open-telemetry/otel-arrow/api/experimental/arrow/v1"
	arrowRecord "github.com/open-telemetry/otel-arrow/pkg/otel/arrow_record"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"github.com/open-telemetry/otel-arrow/collector/compression/zstd"
	"github.com/open-telemetry/otel-arrow/collector/netstats"
	"github.com/open-telemetry/otel-arrow/collector/receiver/otelarrowreceiver/internal/arrow"
	"github.com/open-telemetry/otel-arrow/collector/receiver/otelarrowreceiver/internal/logs"
	"github.com/open-telemetry/otel-arrow/collector/receiver/otelarrowreceiver/internal/metrics"
	"github.com/open-telemetry/otel-arrow/collector/receiver/otelarrowreceiver/internal/trace"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/extension/auth"
	"go.opentelemetry.io/collector/pdata/plog/plogotlp"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
	"go.opentelemetry.io/collector/receiver"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
)

// otlpReceiver is the type that exposes Trace and Metrics reception.
type otlpReceiver struct {
	cfg        *Config
	serverGRPC *grpc.Server

	tracesReceiver  *trace.Receiver
	metricsReceiver *metrics.Receiver
	logsReceiver    *logs.Receiver
	arrowReceiver   *arrow.Receiver
	shutdownWG      sync.WaitGroup

	obsrepGRPC  *receiverhelper.ObsReport
	netReporter *netstats.NetworkReporter

	settings receiver.CreateSettings
}

// newOTelArrowReceiver just creates the OpenTelemetry receiver services. It is the caller's
// responsibility to invoke the respective Start*Reception methods as well
// as the various Stop*Reception methods to end it.
func newOTelArrowReceiver(cfg *Config, set receiver.CreateSettings) (*otlpReceiver, error) {
	netReporter, err := netstats.NewReceiverNetworkReporter(set)
	if err != nil {
		return nil, err
	}
	r := &otlpReceiver{
		cfg:         cfg,
		settings:    set,
		netReporter: netReporter,
	}
	zstd.SetDecoderConfig(cfg.Arrow.Zstd)

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

func (r *otlpReceiver) startGRPCServer(cfg configgrpc.GRPCServerSettings, host component.Host) error {
	r.settings.Logger.Info("Starting GRPC server", zap.String("endpoint", cfg.NetAddr.Endpoint))

	gln, err := cfg.ToListener()
	if err != nil {
		return err
	}
	r.shutdownWG.Add(1)
	go func() {
		defer r.shutdownWG.Done()

		if errGrpc := r.serverGRPC.Serve(gln); errGrpc != nil && !errors.Is(errGrpc, grpc.ErrServerStopped) {
			host.ReportFatalError(errGrpc)
		}
	}()
	return nil
}

func (r *otlpReceiver) startProtocolServers(host component.Host) error {
	var err error
	var serverOpts []grpc.ServerOption

	if r.netReporter != nil {
		serverOpts = append(serverOpts, grpc.StatsHandler(r.netReporter.Handler()))
	}
	r.serverGRPC, err = r.cfg.GRPC.ToServer(host, r.settings.TelemetrySettings, serverOpts...)
	if err != nil {
		return err
	}

	var authServer auth.Server
	if r.cfg.GRPC.Auth != nil {
		authServer, err = r.cfg.GRPC.Auth.GetServerAuthenticator(host.GetExtensions())
		if err != nil {
			return err
		}
	}

	r.arrowReceiver = arrow.New(arrow.Consumers(r), r.settings, r.obsrepGRPC, r.cfg.GRPC, authServer, func() arrowRecord.ConsumerAPI {
		var opts []arrowRecord.Option
		if r.cfg.Arrow.MemoryLimitMiB != 0 {
			// in which case the default is selected in the arrowRecord package.
			opts = append(opts, arrowRecord.WithMemoryLimit(r.cfg.Arrow.MemoryLimitMiB<<20))
		}
		if r.settings.TelemetrySettings.MeterProvider != nil {
			opts = append(opts, arrowRecord.WithMeterProvider(r.settings.TelemetrySettings.MeterProvider, r.settings.TelemetrySettings.MetricsLevel))
		}
		return arrowRecord.NewConsumer(opts...)
	}, r.netReporter)

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
func (r *otlpReceiver) Start(_ context.Context, host component.Host) error {
	return r.startProtocolServers(host)
}

// Shutdown is a method to turn off receiving.
func (r *otlpReceiver) Shutdown(ctx context.Context) error {
	var err error

	if r.serverGRPC != nil {
		r.serverGRPC.GracefulStop()
	}

	r.shutdownWG.Wait()
	return err
}

func (r *otlpReceiver) registerTraceConsumer(tc consumer.Traces) error {
	if tc == nil {
		return component.ErrNilNextConsumer
	}
	r.tracesReceiver = trace.New(tc, r.obsrepGRPC)
	return nil
}

func (r *otlpReceiver) registerMetricsConsumer(mc consumer.Metrics) error {
	if mc == nil {
		return component.ErrNilNextConsumer
	}
	r.metricsReceiver = metrics.New(mc, r.obsrepGRPC)
	return nil
}

func (r *otlpReceiver) registerLogsConsumer(lc consumer.Logs) error {
	if lc == nil {
		return component.ErrNilNextConsumer
	}
	r.logsReceiver = logs.New(lc, r.obsrepGRPC)
	return nil
}

var _ arrow.Consumers = &otlpReceiver{}

func (r *otlpReceiver) Traces() consumer.Traces {
	if r.tracesReceiver == nil {
		return nil
	}
	return r.tracesReceiver.Consumer()
}

func (r *otlpReceiver) Metrics() consumer.Metrics {
	if r.metricsReceiver == nil {
		return nil
	}
	return r.metricsReceiver.Consumer()
}

func (r *otlpReceiver) Logs() consumer.Logs {
	if r.logsReceiver == nil {
		return nil
	}
	return r.logsReceiver.Consumer()
}
