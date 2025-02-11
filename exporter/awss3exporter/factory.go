// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awss3exporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awss3exporter"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awss3exporter/internal/metadata"
)

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

	return &Config{
		QueueSettings: queueCfg,
		S3Uploader: S3UploaderConfig{
			Region:      "us-east-1",
			S3Partition: "minute",
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

	return exporterhelper.NewLogs(ctx, params,
		config,
		s3Exporter.ConsumeLogs,
		exporterhelper.WithStart(s3Exporter.start),
		exporterhelper.WithQueue(cfg.QueueSettings),
	)
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
		return nil, fmt.Errorf("metrics are not supported by sumo_ic output format")
	}

	return exporterhelper.NewMetrics(ctx, params,
		config,
		s3Exporter.ConsumeMetrics,
		exporterhelper.WithStart(s3Exporter.start),
		exporterhelper.WithQueue(cfg.QueueSettings),
	)
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
		return nil, fmt.Errorf("traces are not supported by sumo_ic output format")
	}

	return exporterhelper.NewTraces(ctx,
		params,
		config,
		s3Exporter.ConsumeTraces,
		exporterhelper.WithStart(s3Exporter.start),
		exporterhelper.WithQueue(cfg.QueueSettings),
	)
}

// checkAndCastConfig checks the configuration type and casts it to the S3 exporter Config struct.
func checkAndCastConfig(c component.Config) (*Config, error) {
	cfg, ok := c.(*Config)
	if !ok {
		return nil, fmt.Errorf("config structure is not of type *awss3exporter.Config")
	}
	return cfg, nil
}
