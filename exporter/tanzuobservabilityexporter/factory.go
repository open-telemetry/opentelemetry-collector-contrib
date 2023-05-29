// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

package tanzuobservabilityexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/tanzuobservabilityexporter"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/tanzuobservabilityexporter/internal/metadata"
)

const (
	exporterType = "tanzuobservability"
)

// NewFactory creates a factory for the exporter.
func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		exporterType,
		createDefaultConfig,
		exporter.WithTraces(createTracesExporter, metadata.TracesStability),
		exporter.WithMetrics(createMetricsExporter, metadata.MetricsStability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		QueueSettings: exporterhelper.NewDefaultQueueSettings(),
		RetrySettings: exporterhelper.NewDefaultRetrySettings(),
	}
}

// createTracesExporter implements exporterhelper.CreateTracesExporter and creates
// an exporter for traces using this configuration
func createTracesExporter(
	ctx context.Context,
	set exporter.CreateSettings,
	cfg component.Config,
) (exporter.Traces, error) {
	exp, err := newTracesExporter(set, cfg)
	if err != nil {
		return nil, err
	}

	tobsCfg, ok := cfg.(*Config)
	if !ok {
		return nil, fmt.Errorf("invalid config: %#v", cfg)
	}

	return exporterhelper.NewTracesExporter(
		ctx,
		set,
		cfg,
		exp.pushTraceData,
		exporterhelper.WithQueue(tobsCfg.QueueSettings),
		exporterhelper.WithRetry(tobsCfg.RetrySettings),
		exporterhelper.WithShutdown(exp.shutdown),
	)
}

func createMetricsExporter(
	ctx context.Context,
	set exporter.CreateSettings,
	cfg component.Config,
) (exporter.Metrics, error) {
	tobsCfg, ok := cfg.(*Config)
	if !ok {
		return nil, fmt.Errorf("invalid config: %#v", cfg)
	}
	exp, err := newMetricsExporter(set, tobsCfg, createMetricsConsumer)
	if err != nil {
		return nil, err
	}

	exporter, err := exporterhelper.NewMetricsExporter(
		ctx,
		set,
		cfg,
		exp.pushMetricsData,
		exporterhelper.WithQueue(tobsCfg.QueueSettings),
		exporterhelper.WithRetry(tobsCfg.RetrySettings),
		exporterhelper.WithShutdown(exp.shutdown),
	)
	if err != nil {
		return nil, err
	}

	return exporter, nil
}
