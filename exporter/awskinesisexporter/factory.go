// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

package awskinesisexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awskinesisexporter"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awskinesisexporter/internal/batch"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awskinesisexporter/internal/metadata"
)

const (
	defaultEncoding    = "otlp"
	defaultCompression = "none"
)

// NewFactory creates a factory for Kinesis exporter.
func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		metadata.Type,
		createDefaultConfig,
		exporter.WithTraces(newTracesExporter, metadata.TracesStability),
		exporter.WithMetrics(newMetricsExporter, metadata.MetricsStability),
		exporter.WithLogs(newLogsExporter, metadata.LogsStability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		TimeoutSettings: exporterhelper.NewDefaultTimeoutSettings(),
		BackOffConfig:   configretry.NewDefaultBackOffConfig(),
		QueueSettings:   exporterhelper.NewDefaultQueueSettings(),
		Encoding: Encoding{
			Name:        defaultEncoding,
			Compression: defaultCompression,
		},
		AWS: AWSConfig{
			Region: "us-west-2",
		},
		MaxRecordsPerBatch: batch.MaxBatchedRecords,
		MaxRecordSize:      batch.MaxRecordSize,
	}
}

func newTracesExporter(ctx context.Context, params exporter.CreateSettings, conf component.Config) (exporter.Traces, error) {
	exp, err := createExporter(ctx, conf, params.Logger)
	if err != nil {
		return nil, err
	}
	c := conf.(*Config)
	return exporterhelper.NewTracesExporter(
		ctx,
		params,
		conf,
		exp.consumeTraces,
		exporterhelper.WithStart(exp.start),
		exporterhelper.WithTimeout(c.TimeoutSettings),
		exporterhelper.WithRetry(c.BackOffConfig),
		exporterhelper.WithQueue(c.QueueSettings),
	)
}

func newMetricsExporter(ctx context.Context, params exporter.CreateSettings, conf component.Config) (exporter.Metrics, error) {
	exp, err := createExporter(ctx, conf, params.Logger)
	if err != nil {
		return nil, err
	}
	c := conf.(*Config)
	return exporterhelper.NewMetricsExporter(
		ctx,
		params,
		c,
		exp.consumeMetrics,
		exporterhelper.WithStart(exp.start),
		exporterhelper.WithTimeout(c.TimeoutSettings),
		exporterhelper.WithRetry(c.BackOffConfig),
		exporterhelper.WithQueue(c.QueueSettings),
	)
}

func newLogsExporter(ctx context.Context, params exporter.CreateSettings, conf component.Config) (exporter.Logs, error) {
	exp, err := createExporter(ctx, conf, params.Logger)
	if err != nil {
		return nil, err
	}
	c := conf.(*Config)
	return exporterhelper.NewLogsExporter(
		ctx,
		params,
		c,
		exp.consumeLogs,
		exporterhelper.WithStart(exp.start),
		exporterhelper.WithTimeout(c.TimeoutSettings),
		exporterhelper.WithRetry(c.BackOffConfig),
		exporterhelper.WithQueue(c.QueueSettings),
	)
}
