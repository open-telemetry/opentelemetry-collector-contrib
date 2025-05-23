// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelarrowexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/otelarrowexporter"

import (
	"context"
	"time"

	arrowpb "github.com/open-telemetry/otel-arrow/api/experimental/arrow/v1"
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
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/otelarrow/compression/zstd"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/otelarrow/netstats"
)

// NewFactory creates a factory for OTLP exporter.
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
	// These defaults are taken from the experimental setup used
	// in the blog post covering Phase 1 performance results.  These
	// were the defaults used in the concurrentbatchprocessor, too.
	queueCfg := exporterhelper.NewDefaultQueueConfig()
	queueCfg.BlockOnOverflow = true
	queueCfg.Sizer = exporterhelper.RequestSizerTypeItems
	queueCfg.Batch = &exporterhelper.BatchConfig{
		FlushTimeout: time.Second,
		MinSize:      1000,
		MaxSize:      1500,
	}
	// The default is configured in items, this value represents
	// 60-100 concurrent batches.
	queueCfg.QueueSize = 100000
	// This enables by default an appropriate number of consumers
	// Note for this exporter the consumer's role is to take from
	// the queue and call into an Arrow stream. When the exporter
	// falls back to OTLP, this is the number of concurrent OTLP
	// exports.
	queueCfg.NumConsumers = int(queueCfg.QueueSize / queueCfg.Batch.MinSize)

	return &Config{
		TimeoutSettings: exporterhelper.NewDefaultTimeoutConfig(),
		RetryConfig:     configretry.NewDefaultBackOffConfig(),
		QueueSettings:   queueCfg,
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
		Arrow: ArrowConfig{
			NumStreams:        arrow.DefaultNumStreams,
			MaxStreamLifetime: arrow.DefaultMaxStreamLifetime,

			Zstd:        zstd.DefaultEncoderConfig(),
			Prioritizer: arrow.DefaultPrioritizer,

			// Note the default payload compression is
			PayloadCompression: arrow.DefaultPayloadCompression,
		},
	}
}

func helperOptions(e exp, qbs exporterhelper.QueueBatchSettings) []exporterhelper.Option {
	cfg := e.getConfig().(*Config)
	return []exporterhelper.Option{
		exporterhelper.WithCapabilities(consumer.Capabilities{MutatesData: false}),
		exporterhelper.WithTimeout(cfg.TimeoutSettings),
		exporterhelper.WithRetry(cfg.RetryConfig),
		exporterhelper.WithQueueBatch(cfg.QueueSettings, qbs),
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
	set exporter.Settings,
	cfg component.Config,
) (exporter.Traces, error) {
	e, err := newMetadataExporter(cfg, set, createArrowTracesStream)
	if err != nil {
		return nil, err
	}
	return exporterhelper.NewTraces(ctx, e.getSettings(), e.getConfig(),
		e.pushTraces,
		helperOptions(e, exporterhelper.NewTracesQueueBatchSettings())...,
	)
}

func createArrowMetricsStream(conn *grpc.ClientConn) arrow.StreamClientFunc {
	return arrow.MakeAnyStreamClient(arrowMetricsMethod, arrowpb.NewArrowMetricsServiceClient(conn).ArrowMetrics)
}

func createMetricsExporter(
	ctx context.Context,
	set exporter.Settings,
	cfg component.Config,
) (exporter.Metrics, error) {
	e, err := newMetadataExporter(cfg, set, createArrowMetricsStream)
	if err != nil {
		return nil, err
	}
	return exporterhelper.NewMetrics(ctx, e.getSettings(), e.getConfig(),
		e.pushMetrics,
		helperOptions(e, exporterhelper.NewMetricsQueueBatchSettings())...,
	)
}

func createArrowLogsStream(conn *grpc.ClientConn) arrow.StreamClientFunc {
	return arrow.MakeAnyStreamClient(arrowLogsMethod, arrowpb.NewArrowLogsServiceClient(conn).ArrowLogs)
}

func createLogsExporter(
	ctx context.Context,
	set exporter.Settings,
	cfg component.Config,
) (exporter.Logs, error) {
	e, err := newMetadataExporter(cfg, set, createArrowLogsStream)
	if err != nil {
		return nil, err
	}
	return exporterhelper.NewLogs(ctx, e.getSettings(), e.getConfig(),
		e.pushLogs,
		helperOptions(e, exporterhelper.NewLogsQueueBatchSettings())...,
	)
}
