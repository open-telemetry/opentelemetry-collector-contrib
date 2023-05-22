// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package skywalkingexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/skywalkingexporter"

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	metricpb "skywalking.apache.org/repo/goapi/collect/language/agent/v3"
	logpb "skywalking.apache.org/repo/goapi/collect/logging/v3"
)

// See https://godoc.org/google.golang.org/grpc#ClientConn.NewStream
// why we need to keep the cancel func to cancel the stream
type logsClientWithCancel struct {
	cancel context.CancelFunc
	tsec   logpb.LogReportService_CollectClient
}

type metricsClientWithCancel struct {
	cancel context.CancelFunc
	tsec   metricpb.MeterReportService_CollectBatchClient
}

type swExporter struct {
	cfg *Config
	// gRPC clients and connection.
	logSvcClient    logpb.LogReportServiceClient
	metricSvcClient metricpb.MeterReportServiceClient
	// In any of the channels we keep always NumStreams object (sometimes nil),
	// to make sure we don't open more than NumStreams RPCs at any moment.
	logsClients    chan *logsClientWithCancel
	metricsClients chan *metricsClientWithCancel
	grpcClientConn *grpc.ClientConn
	metadata       metadata.MD

	settings component.TelemetrySettings
}

func newSwExporter(_ context.Context, cfg *Config, settings component.TelemetrySettings) *swExporter {
	oce := &swExporter{
		cfg:      cfg,
		metadata: metadata.New(nil),
		settings: settings,
	}
	for k, v := range cfg.GRPCClientSettings.Headers {
		oce.metadata.Set(k, string(v))
	}
	return oce
}

// start creates the gRPC client Connection
func (oce *swExporter) start(ctx context.Context, host component.Host) error {
	clientConn, err := oce.cfg.GRPCClientSettings.ToClientConn(ctx, host, oce.settings)
	if err != nil {
		return err
	}

	oce.grpcClientConn = clientConn

	if oce.logsClients != nil {
		oce.logSvcClient = logpb.NewLogReportServiceClient(oce.grpcClientConn)
		// Try to create rpc clients now.
		for i := 0; i < oce.cfg.NumStreams; i++ {
			// Populate the channel with NumStreams nil RPCs to keep the number of streams
			// constant in the channel.
			oce.logsClients <- nil
		}
	}

	if oce.metricsClients != nil {
		oce.metricSvcClient = metricpb.NewMeterReportServiceClient(oce.grpcClientConn)
		// Try to create rpc clients now.
		for i := 0; i < oce.cfg.NumStreams; i++ {
			// Populate the channel with NumStreams nil RPCs to keep the number of streams
			// constant in the channel.
			oce.metricsClients <- nil
		}
	}
	return nil
}

func (oce *swExporter) shutdown(context.Context) error {
	if oce.grpcClientConn == nil {
		return nil
	}
	if oce.logsClients != nil {
		// First remove all the clients from the channel.
		for i := 0; i < oce.cfg.NumStreams; i++ {
			<-oce.logsClients
		}
		// Now close the channel
		close(oce.logsClients)
	}
	return oce.grpcClientConn.Close()
}

func newLogsExporter(ctx context.Context, cfg *Config, settings component.TelemetrySettings) *swExporter {
	oce := newSwExporter(ctx, cfg, settings)
	oce.logsClients = make(chan *logsClientWithCancel, oce.cfg.NumStreams)
	return oce
}

func newMetricsExporter(ctx context.Context, cfg *Config, settings component.TelemetrySettings) *swExporter {
	oce := newSwExporter(ctx, cfg, settings)
	oce.metricsClients = make(chan *metricsClientWithCancel, oce.cfg.NumStreams)
	return oce
}

func (oce *swExporter) pushLogs(_ context.Context, td plog.Logs) error {
	// Get first available log Client.
	tClient, ok := <-oce.logsClients
	if !ok {
		return errors.New("failed to push logs, Skywalking exporter was already stopped")
	}

	if tClient == nil {
		var err error
		tClient, err = oce.createLogServiceRPC()
		if err != nil {
			// Cannot create an RPC, put back nil to keep the number of streams constant.
			oce.logsClients <- nil
			return err
		}
	}

	for _, logData := range logRecordToLogData(td) {
		err := tClient.tsec.Send(logData)
		if err != nil {
			// Error received, cancel the context used to create the RPC to free all resources,
			// put back nil to keep the number of streams constant.
			tClient.cancel()
			oce.logsClients <- nil
			return err
		}
	}

	oce.logsClients <- tClient
	return nil
}

func (oce *swExporter) pushMetrics(_ context.Context, td pmetric.Metrics) error {
	// Get first available metric Client.
	tClient, ok := <-oce.metricsClients
	if !ok {
		return errors.New("failed to push metrics, Skywalking exporter was already stopped")
	}

	if tClient == nil {
		var err error
		tClient, err = oce.createMetricServiceRPC()
		if err != nil {
			// Cannot create an RPC, put back nil to keep the number of streams constant.
			oce.metricsClients <- nil
			return err
		}
	}

	err := tClient.tsec.Send(metricsRecordToMetricData(td))
	if err != nil {
		// Error received, cancel the context used to create the RPC to free all resources,
		// put back nil to keep the number of streams constant.
		tClient.cancel()
		oce.metricsClients <- nil
		return err
	}
	oce.metricsClients <- tClient
	return nil
}

func (oce *swExporter) createLogServiceRPC() (*logsClientWithCancel, error) {
	// Initiate the log service by sending over node identifier info.
	ctx, cancel := context.WithCancel(context.Background())
	if len(oce.cfg.Headers) > 0 {
		ctx = metadata.NewOutgoingContext(ctx, oce.metadata.Copy())
	}
	// Cannot use grpc.WaitForReady(cfg.WaitForReady) because will block forever.
	logClient, err := oce.logSvcClient.Collect(ctx)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("LogServiceClient: %w", err)
	}
	return &logsClientWithCancel{cancel: cancel, tsec: logClient}, nil
}

func (oce *swExporter) createMetricServiceRPC() (*metricsClientWithCancel, error) {
	// Initiate the metric service by sending over node identifier info.
	ctx, cancel := context.WithCancel(context.Background())
	if len(oce.cfg.Headers) > 0 {
		ctx = metadata.NewOutgoingContext(ctx, oce.metadata.Copy())
	}
	// Cannot use grpc.WaitForReady(cfg.WaitForReady) because will block forever.
	metricClient, err := oce.metricSvcClient.CollectBatch(ctx)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("MetricServiceClient: %w", err)
	}
	return &metricsClientWithCancel{cancel: cancel, tsec: metricClient}, nil
}
