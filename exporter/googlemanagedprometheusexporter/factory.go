// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlemanagedprometheusexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/googlemanagedprometheusexporter"

import (
	"context"
	"time"

	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

const (
	// The value of "type" key in configuration.
	typeStr = "googlemanagedprometheus"
	// The stability level of the exporter.
	stability      = component.StabilityLevelBeta
	defaultTimeout = 12 * time.Second // Consistent with Cloud Monitoring's timeout
)

// NewFactory creates a factory for the googlemanagedprometheus exporter
func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		typeStr,
		createDefaultConfig,
		exporter.WithMetrics(createMetricsExporter, stability),
	)
}

// createDefaultConfig creates the default configuration for exporter.
func createDefaultConfig() component.Config {
	retrySettings := exporterhelper.NewDefaultRetrySettings()
	retrySettings.Enabled = false
	return &Config{
		TimeoutSettings: exporterhelper.TimeoutSettings{Timeout: defaultTimeout},
		RetrySettings:   retrySettings,
		QueueSettings:   exporterhelper.NewDefaultQueueSettings(),
	}
}

// createMetricsExporter creates a metrics exporter based on this config.
func createMetricsExporter(
	ctx context.Context,
	params exporter.CreateSettings,
	cfg component.Config) (exporter.Metrics, error) {
	eCfg := cfg.(*Config)
	mExp, err := collector.NewGoogleCloudMetricsExporter(ctx, eCfg.GMPConfig.toCollectorConfig(), params.TelemetrySettings.Logger, params.BuildInfo.Version, eCfg.Timeout)
	if err != nil {
		return nil, err
	}
	return exporterhelper.NewMetricsExporter(
		ctx,
		params,
		cfg,
		mExp.PushMetrics,
		exporterhelper.WithShutdown(mExp.Shutdown),
		// Disable exporterhelper Timeout, since we are using a custom mechanism
		// within exporter itself
		exporterhelper.WithTimeout(exporterhelper.TimeoutSettings{Timeout: 0}),
		exporterhelper.WithQueue(eCfg.QueueSettings),
		exporterhelper.WithRetry(eCfg.RetrySettings))
}
