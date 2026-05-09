// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate make mdatagen

package loadbalancingexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/exporterhelper/xexporterhelper"
	"go.opentelemetry.io/collector/exporter/otlpexporter"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter/internal/metadata"
)

const (
	zapEndpointKey = "endpoint"
)

// NewFactory creates a factory for the exporter.
func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		metadata.Type,
		createDefaultConfig,
		exporter.WithTraces(createTracesExporter, metadata.TracesStability),
		exporter.WithLogs(createLogsExporter, metadata.LogsStability),
		exporter.WithMetrics(createMetricsExporter, metadata.MetricsStability),
	)
}

func createDefaultConfig() component.Config {
	otlpFactory := otlpexporter.NewFactory()
	otlpDefaultCfg := otlpFactory.CreateDefaultConfig().(*otlpexporter.Config)
	otlpDefaultCfg.ClientConfig.Endpoint = "placeholder:4317"

	return &Config{
		// By default we disable resilience options on loadbalancing exporter level
		// to maintain compatibility with workflow in previous versions
		Protocol: Protocol{
			OTLP: *otlpDefaultCfg,
		},
		QueueSettings: QueueSettings{
			QueueConfig:        configoptional.None[exporterhelper.QueueBatchConfig](),
			PayloadCompression: QueuePayloadCompressionNone,
		},
		CentralQueue: CentralQueueConfig{
			Enabled:                      false,
			PayloadCompression:           QueuePayloadCompressionZstd,
			MaxUncompressedBatchBytes:    defaultCentralQueueMaxUncompressedBatchBytes,
			MaxInflightUncompressedBytes: defaultCentralQueueMaxInflightBytes,
			TargetCompressedBytes:        defaultCentralQueueTargetCompressedBytes,
			MaxBatchDelay:                defaultCentralQueueMaxBatchDelay,
			LaneCount:                    defaultCentralQueueLaneCount,
			NumConsumers:                 defaultCentralQueueNumConsumers,
		},
		LogBatcher: LogBatcherConfig{
			Enabled:            false,
			MaxRecords:         defaultLogBatchMaxRecords,
			MaxBytes:           defaultLogBatchMaxBytes,
			FlushInterval:      defaultLogBatchFlushTimeout,
			PayloadCompression: QueuePayloadCompressionNone,
		},
		MetricBatcher: MetricBatcherConfig{
			Enabled:                  false,
			MaxDataPoints:            defaultMetricBatchMaxDataPoints,
			MaxBytes:                 defaultMetricBatchMaxBytes,
			FlushInterval:            defaultMetricBatchFlushTimeout,
			MaxRetryBufferMultiplier: defaultMetricBatchRetryBufferMultiplier,
			PayloadCompression:       QueuePayloadCompressionNone,
		},
		EndpointHealth: EndpointHealthConfig{
			Enabled:            false,
			QuarantineDuration: defaultEndpointHealthQuarantineDuration,
			RerouteOnFailure:   true,
			MaxRerouteAttempts: 1,
			ActiveProbe: EndpointHealthActiveProbeConfig{
				Enabled:        false,
				Type:           EndpointHealthActiveProbeTypeTCPConnect,
				Interval:       defaultEndpointHealthActiveProbeInterval,
				Timeout:        defaultEndpointHealthActiveProbeTimeout,
				Jitter:         defaultEndpointHealthActiveProbeJitter,
				MaxConcurrency: defaultEndpointHealthActiveProbeMaxConcurrency,
				Fall:           defaultEndpointHealthActiveProbeFall,
				Rise:           defaultEndpointHealthActiveProbeRise,
			},
		},
	}
}

func buildExporterConfig(cfg *Config, endpoint string) otlpexporter.Config {
	oCfg := cfg.Protocol.OTLP
	oCfg.ClientConfig.Endpoint = endpoint
	if cfg.CentralQueue.Enabled {
		oCfg.QueueConfig = configoptional.None[exporterhelper.QueueBatchConfig]()
	}

	return oCfg
}

func buildExporterSettings(typ component.Type, params exporter.Settings, endpoint string) exporter.Settings {
	if name := params.ID.Name(); name != "" {
		params.ID = component.NewIDWithName(typ, name)
	} else {
		params.ID = component.NewID(typ)
	}
	telemetry := params.TelemetrySettings
	telemetry.MeterProvider = metricnoop.NewMeterProvider()
	telemetry.TracerProvider = tracenoop.NewTracerProvider()
	params.Logger = params.Logger.With(zap.String(zapEndpointKey, endpoint))
	telemetry.Logger = params.Logger
	params.TelemetrySettings = telemetry
	return params
}

func buildExporterResilienceOptions(
	options []exporterhelper.Option,
	cfg *Config,
	payloadCodec *queuePayloadCodec,
	qbs xexporterhelper.QueueBatchSettings,
) []exporterhelper.Option {
	if cfg.TimeoutSettings.Timeout > 0 {
		options = append(options, exporterhelper.WithTimeout(cfg.TimeoutSettings))
	}
	if queueCfg, ok := queueConfigForExport(cfg); ok {
		if payloadCodec != nil && qbs.Encoding != nil {
			qbs.Encoding = payloadCodecEncoding{
				encoding: qbs.Encoding,
				codec:    payloadCodec,
			}
		}
		options = append(options, xexporterhelper.WithQueueBatch(queueCfg, qbs))
	}
	if cfg.Enabled {
		options = append(options, exporterhelper.WithRetry(cfg.BackOffConfig))
	}

	return options
}

func createTracesExporter(ctx context.Context, params exporter.Settings, cfg component.Config) (exporter.Traces, error) {
	c := cfg.(*Config)
	exp, err := newTracesExporter(params, cfg)
	if err != nil {
		return nil, fmt.Errorf("cannot configure loadbalancing traces exporter: %w", err)
	}
	payloadCodec := newQueuePayloadCodecIfEnabled(c)

	options := []exporterhelper.Option{
		exporterhelper.WithStart(exp.Start),
		exporterhelper.WithShutdown(shutdownWithCodec(component.ShutdownFunc(exp.Shutdown), payloadCodec)),
		exporterhelper.WithCapabilities(exp.Capabilities()),
	}
	qbs := xexporterhelper.NewTracesQueueBatchSettings()

	return exporterhelper.NewTraces(
		ctx,
		params,
		cfg,
		exp.ConsumeTraces,
		buildExporterResilienceOptions(options, c, payloadCodec, qbs)...,
	)
}

func createLogsExporter(ctx context.Context, params exporter.Settings, cfg component.Config) (exporter.Logs, error) {
	c := cfg.(*Config)
	exporter, err := newLogsExporter(params, cfg)
	if err != nil {
		return nil, fmt.Errorf("cannot configure loadbalancing logs exporter: %w", err)
	}
	payloadCodec := newQueuePayloadCodecIfEnabled(c)

	options := []exporterhelper.Option{
		exporterhelper.WithStart(exporter.Start),
		exporterhelper.WithShutdown(shutdownWithCodec(component.ShutdownFunc(exporter.Shutdown), payloadCodec)),
		exporterhelper.WithCapabilities(exporter.Capabilities()),
	}
	qbs := xexporterhelper.NewLogsQueueBatchSettings()

	return exporterhelper.NewLogs(
		ctx,
		params,
		cfg,
		exporter.ConsumeLogs,
		buildExporterResilienceOptions(options, c, payloadCodec, qbs)...,
	)
}

func createMetricsExporter(ctx context.Context, params exporter.Settings, cfg component.Config) (exporter.Metrics, error) {
	c := cfg.(*Config)
	exporter, err := newMetricsExporter(params, cfg)
	if err != nil {
		return nil, fmt.Errorf("cannot configure loadbalancing metrics exporter: %w", err)
	}
	payloadCodec := newQueuePayloadCodecIfEnabled(c)

	options := []exporterhelper.Option{
		exporterhelper.WithStart(exporter.Start),
		exporterhelper.WithShutdown(shutdownWithCodec(component.ShutdownFunc(exporter.Shutdown), payloadCodec)),
		exporterhelper.WithCapabilities(exporter.Capabilities()),
	}
	qbs := xexporterhelper.NewMetricsQueueBatchSettings()

	return exporterhelper.NewMetrics(
		ctx,
		params,
		cfg,
		exporter.ConsumeMetrics,
		buildExporterResilienceOptions(options, c, payloadCodec, qbs)...,
	)
}

func newQueuePayloadCodecIfEnabled(cfg *Config) *queuePayloadCodec {
	if !cfg.QueueSettings.QueueConfig.HasValue() {
		return nil
	}
	if cfg.QueueSettings.PayloadCompression == "" {
		return nil
	}

	return newQueuePayloadCodec(cfg.QueueSettings.PayloadCompression)
}

func shutdownWithCodec(shutdown component.ShutdownFunc, codec *queuePayloadCodec) component.ShutdownFunc {
	if codec == nil {
		return shutdown
	}
	return func(ctx context.Context) error {
		return multierr.Append(shutdown.Shutdown(ctx), codec.Close())
	}
}

func queueConfigForExport(cfg *Config) (configoptional.Optional[exporterhelper.QueueBatchConfig], bool) {
	if !cfg.QueueSettings.QueueConfig.HasValue() {
		return configoptional.None[exporterhelper.QueueBatchConfig](), false
	}

	queueCfg := *cfg.QueueSettings.QueueConfig.Get()

	return configoptional.Some(queueCfg), true
}
