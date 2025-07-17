// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opencensusexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opencensusexporter"

import (
	"context"
	"errors"
	"fmt"

	commonpb "github.com/census-instrumentation/opencensus-proto/gen-go/agent/common/v1"
	agentmetricspb "github.com/census-instrumentation/opencensus-proto/gen-go/agent/metrics/v1"
	agenttracepb "github.com/census-instrumentation/opencensus-proto/gen-go/agent/trace/v1"
	resourcepb "github.com/census-instrumentation/opencensus-proto/gen-go/resource/v1"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/opencensus"
)

// See https://godoc.org/google.golang.org/grpc#ClientConn.NewStream
// why we need to keep the cancel func to cancel the stream
type tracesClientWithCancel struct {
	cancel context.CancelFunc
	tsec   agenttracepb.TraceService_ExportClient
}

// See https://godoc.org/google.golang.org/grpc#ClientConn.NewStream
// why we need to keep the cancel func to cancel the stream
type metricsClientWithCancel struct {
	cancel context.CancelFunc
	msec   agentmetricspb.MetricsService_ExportClient
}

type ocExporter struct {
	cfg *Config
	// gRPC clients and connection.
	traceSvcClient   agenttracepb.TraceServiceClient
	metricsSvcClient agentmetricspb.MetricsServiceClient
	// In any of the channels we keep always NumWorkers object (sometimes nil),
	// to make sure we don't open more than NumWorkers RPCs at any moment.
	tracesClients  chan *tracesClientWithCancel
	metricsClients chan *metricsClientWithCancel
	grpcClientConn *grpc.ClientConn
	metadata       metadata.MD

	settings component.TelemetrySettings
}

func newOcExporter(_ context.Context, cfg *Config, settings component.TelemetrySettings) (*ocExporter, error) {
	if cfg.Endpoint == "" {
		return nil, errors.New("OpenCensus exporter cfg requires an Endpoint")
	}

	if cfg.NumWorkers <= 0 {
		return nil, errors.New("OpenCensus exporter cfg requires at least one worker")
	}

	oce := &ocExporter{
		cfg:      cfg,
		metadata: metadata.New(nil),
		settings: settings,
	}
	for k, v := range cfg.Headers {
		oce.metadata.Set(k, string(v))
	}
	return oce, nil
}

// start creates the gRPC client Connection
func (oce *ocExporter) start(ctx context.Context, host component.Host) error {
	clientConn, err := oce.cfg.ToClientConn(ctx, host, oce.settings)
	if err != nil {
		return err
	}
	oce.grpcClientConn = clientConn

	if oce.tracesClients != nil {
		oce.traceSvcClient = agenttracepb.NewTraceServiceClient(oce.grpcClientConn)
		// Try to create rpc clients now.
		for i := 0; i < oce.cfg.NumWorkers; i++ {
			// Populate the channel with NumWorkers nil RPCs to keep the number of workers
			// constant in the channel.
			oce.tracesClients <- nil
		}
	}

	if oce.metricsClients != nil {
		oce.metricsSvcClient = agentmetricspb.NewMetricsServiceClient(oce.grpcClientConn)
		// Try to create rpc clients now.
		for i := 0; i < oce.cfg.NumWorkers; i++ {
			// Populate the channel with NumWorkers nil RPCs to keep the number of workers
			// constant in the channel.
			oce.metricsClients <- nil
		}
	}
	return nil
}

func (oce *ocExporter) shutdown(context.Context) error {
	if oce.grpcClientConn == nil {
		return nil
	}
	if oce.tracesClients != nil {
		// First remove all the clients from the channel.
		for i := 0; i < oce.cfg.NumWorkers; i++ {
			<-oce.tracesClients
		}
		// Now close the channel
		close(oce.tracesClients)
	}
	if oce.metricsClients != nil {
		// First remove all the clients from the channel.
		for i := 0; i < oce.cfg.NumWorkers; i++ {
			<-oce.metricsClients
		}
		// Now close the channel
		close(oce.metricsClients)
	}
	return oce.grpcClientConn.Close()
}

func newTracesExporter(ctx context.Context, cfg *Config, settings component.TelemetrySettings) (*ocExporter, error) {
	oce, err := newOcExporter(ctx, cfg, settings)
	if err != nil {
		return nil, err
	}
	oce.tracesClients = make(chan *tracesClientWithCancel, oce.cfg.NumWorkers)
	return oce, nil
}

func newMetricsExporter(ctx context.Context, cfg *Config, settings component.TelemetrySettings) (*ocExporter, error) {
	oce, err := newOcExporter(ctx, cfg, settings)
	if err != nil {
		return nil, err
	}
	oce.metricsClients = make(chan *metricsClientWithCancel, oce.cfg.NumWorkers)
	return oce, nil
}

func (oce *ocExporter) pushTraces(_ context.Context, td ptrace.Traces) error {
	// Get first available trace Client.
	tClient, ok := <-oce.tracesClients
	if !ok {
		err := errors.New("failed to push traces, OpenCensus exporter was already stopped")
		return err
	}

	// In any of the metricsClients channel we keep always NumWorkers object (sometimes nil),
	// to make sure we don't open more than NumWorkers RPCs at any moment.
	// Here check if the client is nil and create a new one if that is the case. A nil
	// object means that an error happened: could not connect, service went down, etc.
	if tClient == nil {
		var err error
		tClient, err = oce.createTraceServiceRPC()
		if err != nil {
			// Cannot create an RPC, put back nil to keep the number of workers constant.
			oce.tracesClients <- nil
			return err
		}
	}

	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		node, resource, spans := opencensus.ResourceSpansToOC(rss.At(i))
		// This is a hack because OC protocol expects a Node for the initial message.
		if node == nil {
			node = &commonpb.Node{}
		}
		if resource == nil {
			resource = &resourcepb.Resource{}
		}
		req := &agenttracepb.ExportTraceServiceRequest{
			Spans:    spans,
			Resource: resource,
			Node:     node,
		}
		if err := tClient.tsec.Send(req); err != nil {
			// Error received, cancel the context used to create the RPC to free all resources,
			// put back nil to keep the number of workers constant.
			tClient.cancel()
			oce.tracesClients <- nil
			return err
		}
	}
	oce.tracesClients <- tClient
	return nil
}

func (oce *ocExporter) pushMetrics(_ context.Context, md pmetric.Metrics) error {
	// Get first available mClient.
	mClient, ok := <-oce.metricsClients
	if !ok {
		err := errors.New("failed to push metrics, OpenCensus exporter was already stopped")
		return err
	}

	// In any of the metricsClients channel we keep always NumWorkers object (sometimes nil),
	// to make sure we don't open more than NumWorkers RPCs at any moment.
	// Here check if the client is nil and create a new one if that is the case. A nil
	// object means that an error happened: could not connect, service went down, etc.
	if mClient == nil {
		var err error
		mClient, err = oce.createMetricsServiceRPC()
		if err != nil {
			// Cannot create an RPC, put back nil to keep the number of workers constant.
			oce.metricsClients <- nil
			return err
		}
	}

	rms := md.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		ocReq := agentmetricspb.ExportMetricsServiceRequest{}
		ocReq.Node, ocReq.Resource, ocReq.Metrics = opencensus.ResourceMetricsToOC(rms.At(i))

		// This is a hack because OC protocol expects a Node for the initial message.
		if ocReq.Node == nil {
			ocReq.Node = &commonpb.Node{}
		}
		if ocReq.Resource == nil {
			ocReq.Resource = &resourcepb.Resource{}
		}
		if err := mClient.msec.Send(&ocReq); err != nil {
			// Error received, cancel the context used to create the RPC to free all resources,
			// put back nil to keep the number of workers constant.
			mClient.cancel()
			oce.metricsClients <- nil
			return err
		}
	}
	oce.metricsClients <- mClient
	return nil
}

func (oce *ocExporter) createTraceServiceRPC() (*tracesClientWithCancel, error) {
	// Initiate the trace service by sending over node identifier info.
	ctx, cancel := context.WithCancel(context.Background())
	if len(oce.cfg.Headers) > 0 {
		ctx = metadata.NewOutgoingContext(ctx, oce.metadata.Copy())
	}
	// Cannot use grpc.WaitForReady(cfg.WaitForReady) because will block forever.
	traceClient, err := oce.traceSvcClient.Export(ctx)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("TraceServiceClient: %w", err)
	}
	return &tracesClientWithCancel{cancel: cancel, tsec: traceClient}, nil
}

func (oce *ocExporter) createMetricsServiceRPC() (*metricsClientWithCancel, error) {
	// Initiate the trace service by sending over node identifier info.
	ctx, cancel := context.WithCancel(context.Background())
	if len(oce.cfg.Headers) > 0 {
		ctx = metadata.NewOutgoingContext(ctx, oce.metadata.Copy())
	}
	// Cannot use grpc.WaitForReady(cfg.WaitForReady) because will block forever.
	metricsClient, err := oce.metricsSvcClient.Export(ctx)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("MetricsServiceClient: %w", err)
	}
	return &metricsClientWithCancel{cancel: cancel, msec: metricsClient}, nil
}
