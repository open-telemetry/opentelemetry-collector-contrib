// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awss3exporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awss3exporter"

import (
	"context"

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
	return &Config{
		S3Uploader: S3UploaderConfig{
			Region:      "us-east-1",
			S3Partition: "minute",
		},
		MarshalerName: "otlp_json",
	}
}

func createLogsExporter(ctx context.Context,
	params exporter.CreateSettings,
	config component.Config) (exporter.Logs, error) {

	s3Exporter, err := newS3Exporter(config.(*Config), params)
	if err != nil {
		return nil, err
	}

	return exporterhelper.NewLogsExporter(ctx, params,
		config,
		s3Exporter.ConsumeLogs)
}

func createMetricsExporter(ctx context.Context,
	params exporter.CreateSettings,
	config component.Config) (exporter.Metrics, error) {

	s3Exporter, err := newS3Exporter(config.(*Config), params)
	if err != nil {
		return nil, err
	}

	return exporterhelper.NewMetricsExporter(ctx, params,
		config,
		s3Exporter.ConsumeMetrics)
}

func createTracesExporter(ctx context.Context,
	params exporter.CreateSettings,
	config component.Config) (exporter.Traces, error) {

	s3Exporter, err := newS3Exporter(config.(*Config), params)
	if err != nil {
		return nil, err
	}

	return exporterhelper.NewTracesExporter(ctx,
		params,
		config,
		s3Exporter.ConsumeTraces)
}
