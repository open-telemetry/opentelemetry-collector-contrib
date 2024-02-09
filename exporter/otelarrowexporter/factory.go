// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelarrowexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/otelarrowexporter"

import (
	"context"
	"runtime"
	"time"

	arrowpb "github.com/open-telemetry/otel-arrow/api/experimental/arrow/v1"
	"github.com/open-telemetry/otel-arrow/collector/compression/zstd"
	"github.com/open-telemetry/otel-arrow/collector/netstats"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configcompression"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"google.golang.org/grpc"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/otelarrowexporter/internal/arrow"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/otelarrowexporter/internal/metadata"
)

// NewFactory creates a factory for OTel-Arrow exporter.
func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		metadata.Type,
		createDefaultConfig,
		exporter.WithTraces(createTracesExporter, metadata.TracesStability),
		exporter.WithMetrics(createMetricsExporter, metadata.MetricsStability),
		exporter.WithLogs(createLogsExporter, metadata.LogsStability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		TimeoutSettings: exporterhelper.NewDefaultTimeoutSettings(),
		RetrySettings:   configretry.NewDefaultBackOffConfig(),
		QueueSettings:   exporterhelper.NewDefaultQueueSettings(),

		ClientConfig: configgrpc.ClientConfig{
			Headers: map[string]configopaque.String{},
			// Default to zstd compression
			Compression: configcompression.TypeZstd,
			// We almost read 0 bytes, so no need to tune ReadBufferSize.
			WriteBufferSize: 512 * 1024,
			// The `configgrpc` default is pick_first,
			// which is not great for OTel Arrow exporters
			// because it concentrates load at a single
			// destination.
			BalancerName: "round_robin",
		},
		Arrow: ArrowSettings{
			NumStreams:        runtime.NumCPU(),
			MaxStreamLifetime: time.Hour,

			Zstd: zstd.DefaultEncoderConfig(),

			// PayloadCompression is off by default because gRPC
			// compression is on by default, above.
			PayloadCompression: "",
		},
	}
}

func (e *baseExporter) helperOptions() []exporterhelper.Option {
	return []exporterhelper.Option{
		exporterhelper.WithCapabilities(consumer.Capabilities{MutatesData: false}),
		exporterhelper.WithTimeout(e.config.TimeoutSettings),
		exporterhelper.WithRetry(e.config.RetrySettings),
		exporterhelper.WithQueue(e.config.QueueSettings),
		exporterhelper.WithStart(e.start),
		exporterhelper.WithShutdown(e.shutdown),
	}
}

func gRPCName(desc grpc.ServiceDesc) string {
	return netstats.GRPCStreamMethodName(desc, desc.Streams[0])
}

var (
	arrowTracesMethod  = gRPCName(arrowpb.ArrowTracesService_ServiceDesc)
	arrowMetricsMethod = gRPCName(arrowpb.ArrowMetricsService_ServiceDesc)
	arrowLogsMethod    = gRPCName(arrowpb.ArrowLogsService_ServiceDesc)
)

func createArrowTracesStream(conn *grpc.ClientConn) arrow.StreamClientFunc {
	return arrow.MakeAnyStreamClient(arrowTracesMethod, arrowpb.NewArrowTracesServiceClient(conn).ArrowTraces)
}

func createTracesExporter(
	ctx context.Context,
	set exporter.CreateSettings,
	cfg component.Config,
) (exporter.Traces, error) {
	exp, err := newExporter(cfg, set, createArrowTracesStream)
	if err != nil {
		return nil, err
	}
	return exporterhelper.NewTracesExporter(ctx, exp.settings, exp.config,
		exp.pushTraces,
		exp.helperOptions()...,
	)
}

func createArrowMetricsStream(conn *grpc.ClientConn) arrow.StreamClientFunc {
	return arrow.MakeAnyStreamClient(arrowMetricsMethod, arrowpb.NewArrowMetricsServiceClient(conn).ArrowMetrics)
}

func createMetricsExporter(
	ctx context.Context,
	set exporter.CreateSettings,
	cfg component.Config,
) (exporter.Metrics, error) {
	exp, err := newExporter(cfg, set, createArrowMetricsStream)
	if err != nil {
		return nil, err
	}
	return exporterhelper.NewMetricsExporter(ctx, exp.settings, exp.config,
		exp.pushMetrics,
		exp.helperOptions()...,
	)
}

func createArrowLogsStream(conn *grpc.ClientConn) arrow.StreamClientFunc {
	return arrow.MakeAnyStreamClient(arrowLogsMethod, arrowpb.NewArrowLogsServiceClient(conn).ArrowLogs)
}

func createLogsExporter(
	ctx context.Context,
	set exporter.CreateSettings,
	cfg component.Config,
) (exporter.Logs, error) {
	exp, err := newExporter(cfg, set, createArrowLogsStream)
	if err != nil {
		return nil, err
	}
	return exporterhelper.NewLogsExporter(ctx, exp.settings, exp.config,
		exp.pushLogs,
		exp.helperOptions()...,
	)
}
