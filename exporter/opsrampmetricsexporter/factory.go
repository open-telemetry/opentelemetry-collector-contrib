// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opsrampmetricsexporter

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/resourcetotelemetry"
)

// Type for this exporter
var typeStr = component.MustNewType("opsrampmetrics")

// NewFactory creates a factory for the OpsRamp Metrics exporter.
func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		typeStr,
		createDefaultConfig,
		exporter.WithMetrics(createMetricsExporter, component.StabilityLevelAlpha),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		QueueSettings:     exporterhelper.NewDefaultQueueConfig(),
		RetrySettings:     configretry.NewDefaultBackOffConfig(),
		TimeoutSettings:   exporterhelper.NewDefaultTimeoutConfig(),
		AddMetricSuffixes: true,
		ResourceToTelemetrySettings: resourcetotelemetry.Settings{
			Enabled: false,
		},
	}
}

// createMetricsExporter creates a metrics exporter based on this config.
func createMetricsExporter(
	ctx context.Context,
	set exporter.Settings,
	cfg component.Config,
) (exporter.Metrics, error) {
	eCfg := cfg.(*Config)
	exp, err := newOpsRampMetricsExporter(eCfg, set)
	if err != nil {
		return nil, err
	}

	exporter, err := exporterhelper.NewMetricsExporter(
		ctx,
		set,
		cfg,
		exp.pushMetricsData,
		exporterhelper.WithShutdown(exp.shutdown),
		exporterhelper.WithQueue(eCfg.QueueSettings),
		exporterhelper.WithRetry(eCfg.RetrySettings),
		exporterhelper.WithTimeout(eCfg.TimeoutSettings),
	)
	if err != nil {
		return nil, err
	}

	return resourcetotelemetry.WrapMetricsExporter(eCfg.ResourceToTelemetrySettings, exporter), nil
}
