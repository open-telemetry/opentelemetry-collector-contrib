// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awss3exporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awss3exporter"

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awss3exporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/batchperresourceattr"
)

// TODO: Find a place for this to be shared.
type baseMetricsExporter struct {
	component.Component
	consumer.Metrics
}

// TODO: Find a place for this to be shared.
type baseLogsExporter struct {
	component.Component
	consumer.Logs
}

// TODO: Find a place for this to be shared.
type baseTracesExporter struct {
	component.Component
	consumer.Traces
}

// NewFactory creates a factory for S3 exporter.
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
	queueCfg := exporterhelper.NewDefaultQueueConfig()
	queueCfg.Enabled = false
	timeoutCfg := exporterhelper.NewDefaultTimeoutConfig()

	return &Config{
		QueueSettings:   queueCfg,
		TimeoutSettings: timeoutCfg,
		S3Uploader: S3UploaderConfig{
			Region:            "us-east-1",
			S3PartitionFormat: "year=%Y/month=%m/day=%d/hour=%H/minute=%M",
			StorageClass:      "STANDARD",
			RetryMode:         DefaultRetryMode,
			RetryMaxAttempts:  DefaultRetryMaxAttempts,
			RetryMaxBackoff:   DefaultRetryMaxBackoff,
		},
		MarshalerName: "otlp_json",
	}
}

func createLogsExporter(ctx context.Context,
	params exporter.Settings,
	config component.Config,
) (exporter.Logs, error) {
	cfg, err := checkAndCastConfig(config)
	if err != nil {
		return nil, err
	}

	s3Exporter := newS3Exporter(cfg, "logs", params)

	logsExporter, err := exporterhelper.NewLogs(ctx, params,
		config,
		s3Exporter.ConsumeLogs,
		exporterhelper.WithStart(s3Exporter.start),
		exporterhelper.WithQueue(cfg.QueueSettings),
		exporterhelper.WithTimeout(cfg.TimeoutSettings),
	)
	if err != nil {
		return nil, err
	}

	if cfg.ResourceAttrsToS3.S3Prefix == "" {
		return logsExporter, err
	}

	wrapped := &baseLogsExporter{
		Component: logsExporter,
		Logs:      batchperresourceattr.NewBatchPerResourceLogs(cfg.ResourceAttrsToS3.S3Prefix, logsExporter),
	}
	return wrapped, nil
}

func createMetricsExporter(ctx context.Context,
	params exporter.Settings,
	config component.Config,
) (exporter.Metrics, error) {
	cfg, err := checkAndCastConfig(config)
	if err != nil {
		return nil, err
	}

	s3Exporter := newS3Exporter(cfg, "metrics", params)

	if config.(*Config).MarshalerName == SumoIC {
		return nil, errors.New("metrics are not supported by sumo_ic output format")
	}

	metricsExporter, err := exporterhelper.NewMetrics(ctx, params,
		config,
		s3Exporter.ConsumeMetrics,
		exporterhelper.WithStart(s3Exporter.start),
		exporterhelper.WithQueue(cfg.QueueSettings),
		exporterhelper.WithTimeout(cfg.TimeoutSettings),
	)
	if err != nil {
		return nil, err
	}

	if cfg.ResourceAttrsToS3.S3Prefix == "" {
		return metricsExporter, err
	}

	wrapped := &baseMetricsExporter{
		Component: metricsExporter,
		Metrics:   batchperresourceattr.NewBatchPerResourceMetrics(cfg.ResourceAttrsToS3.S3Prefix, metricsExporter),
	}
	return wrapped, nil
}

func createTracesExporter(ctx context.Context,
	params exporter.Settings,
	config component.Config,
) (exporter.Traces, error) {
	cfg, err := checkAndCastConfig(config)
	if err != nil {
		return nil, err
	}

	s3Exporter := newS3Exporter(cfg, "traces", params)

	if config.(*Config).MarshalerName == SumoIC {
		return nil, errors.New("traces are not supported by sumo_ic output format")
	}

	tracesExporter, err := exporterhelper.NewTraces(ctx,
		params,
		config,
		s3Exporter.ConsumeTraces,
		exporterhelper.WithStart(s3Exporter.start),
		exporterhelper.WithQueue(cfg.QueueSettings),
		exporterhelper.WithTimeout(cfg.TimeoutSettings),
	)
	if err != nil {
		return nil, err
	}

	if cfg.ResourceAttrsToS3.S3Prefix == "" {
		return tracesExporter, err
	}

	wrapped := &baseTracesExporter{
		Component: tracesExporter,
		Traces:    batchperresourceattr.NewBatchPerResourceTraces(cfg.ResourceAttrsToS3.S3Prefix, tracesExporter),
	}
	return wrapped, nil
}

// checkAndCastConfig checks the configuration type and casts it to the S3 exporter Config struct.
func checkAndCastConfig(c component.Config) (*Config, error) {
	cfg, ok := c.(*Config)
	if !ok {
		return nil, errors.New("config structure is not of type *awss3exporter.Config")
	}
	return cfg, nil
}
