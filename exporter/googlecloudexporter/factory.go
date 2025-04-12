// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

package googlecloudexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/googlecloudexporter"

import (
	"context"
	"time"

	"github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/featuregate"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/googlecloudexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/googlecloudexporter/internal/resourcemapping"
)

const (
	defaultTimeout = 12 * time.Second // Consistent with Cloud Monitoring's timeout
)

var _ = featuregate.GlobalRegistry().MustRegister(
	"exporter.googlecloud.OTLPDirect",
	featuregate.StageStable,
	featuregate.WithRegisterDescription("When enabled, the googlecloud exporter translates pdata directly to google cloud monitoring's types, rather than first translating to opencensus."),
	featuregate.WithRegisterReferenceURL("https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/7132"),
	featuregate.WithRegisterToVersion("v0.69.0"),
)

var customMonitoredResourcesMetricsGate = featuregate.GlobalRegistry().MustRegister(
	"exporter.googlecloud.CustomMonitoredResources",
	featuregate.StageAlpha,
	featuregate.WithRegisterDescription("When enabled, the googlecloudexporter"+
		" will map the OTLP metrics to the monitored resource type defined by the resource label `gcp.resource_type`."+
		" The MR labels are defined by resource labels with the prefix `gcp.<monitored_resource_type>."),
)

// NewFactory creates a factory for the googlecloud exporter
func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		metadata.Type,
		createDefaultConfig,
		exporter.WithTraces(createTracesExporter, metadata.TracesStability),
		exporter.WithMetrics(createMetricsExporter, metadata.MetricsStability),
		exporter.WithLogs(createLogsExporter, metadata.LogsStability),
	)
}

// createDefaultConfig creates the default configuration for exporter.
func createDefaultConfig() component.Config {
	return &Config{
		TimeoutSettings: exporterhelper.TimeoutConfig{Timeout: defaultTimeout},
		QueueSettings:   exporterhelper.NewDefaultQueueConfig(),
		Config:          collector.DefaultConfig(),
	}
}

func createLogsExporter(
	ctx context.Context,
	params exporter.Settings,
	cfg component.Config,
) (exporter.Logs, error) {
	eCfg := cfg.(*Config)
	logsExporter, err := collector.NewGoogleCloudLogsExporter(ctx, eCfg.Config, params, eCfg.TimeoutSettings.Timeout)
	if err != nil {
		return nil, err
	}
	return exporterhelper.NewLogs(
		ctx,
		params,
		cfg,
		logsExporter.PushLogs,
		exporterhelper.WithStart(logsExporter.Start),
		exporterhelper.WithShutdown(logsExporter.Shutdown),
		// Disable exporterhelper Timeout, since we are using a custom mechanism
		// within exporter itself
		exporterhelper.WithTimeout(exporterhelper.TimeoutConfig{Timeout: 0}),
		exporterhelper.WithQueue(eCfg.QueueSettings),
		exporterhelper.WithCapabilities(consumer.Capabilities{MutatesData: true}),
	)
}

// createTracesExporter creates a trace exporter based on this config.
func createTracesExporter(
	ctx context.Context,
	params exporter.Settings,
	cfg component.Config,
) (exporter.Traces, error) {
	eCfg := cfg.(*Config)
	tExp, err := collector.NewGoogleCloudTracesExporter(ctx, eCfg.Config, params, eCfg.TimeoutSettings.Timeout)
	if err != nil {
		return nil, err
	}
	return exporterhelper.NewTraces(
		ctx,
		params,
		cfg,
		tExp.PushTraces,
		exporterhelper.WithStart(tExp.Start),
		exporterhelper.WithShutdown(tExp.Shutdown),
		// Disable exporterhelper Timeout, since we are using a custom mechanism
		// within exporter itself
		exporterhelper.WithTimeout(exporterhelper.TimeoutConfig{Timeout: 0}),
		exporterhelper.WithQueue(eCfg.QueueSettings),
		exporterhelper.WithCapabilities(consumer.Capabilities{MutatesData: true}),
	)
}

// createMetricsExporter creates a metrics exporter based on this config.
func createMetricsExporter(
	ctx context.Context,
	params exporter.Settings,
	cfg component.Config,
) (exporter.Metrics, error) {
	eCfg := cfg.(*Config)
	config := eCfg.Config
	if customMonitoredResourcesMetricsGate.IsEnabled() {
		config.MetricConfig.MapMonitoredResource = resourcemapping.CustomMonitoredResourceMapping
	}
	mExp, err := collector.NewGoogleCloudMetricsExporter(ctx, config, params, eCfg.TimeoutSettings.Timeout)
	if err != nil {
		return nil, err
	}
	return exporterhelper.NewMetrics(
		ctx,
		params,
		cfg,
		mExp.PushMetrics,
		exporterhelper.WithStart(mExp.Start),
		exporterhelper.WithShutdown(mExp.Shutdown),
		// Disable exporterhelper Timeout, since we are using a custom mechanism
		// within exporter itself
		exporterhelper.WithTimeout(exporterhelper.TimeoutConfig{Timeout: 0}),
		exporterhelper.WithQueue(eCfg.QueueSettings),
		exporterhelper.WithCapabilities(consumer.Capabilities{MutatesData: true}),
	)
}
