// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusexporter"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/resourcetotelemetry"
)

// NewFactory creates a new Prometheus exporter factory.
func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		metadata.Type,
		createDefaultConfig,
		exporter.WithMetrics(createMetricsExporter, metadata.MetricsStability))
}

func createDefaultConfig() component.Config {
	return &Config{
		ServerConfig:      confighttp.NewDefaultServerConfig(),
		ConstLabels:       map[string]string{},
		SendTimestamps:    false,
		MetricExpiration:  time.Minute * 5,
		EnableOpenMetrics: false,
		AddMetricSuffixes: true,
	}
}

func createMetricsExporter(
	ctx context.Context,
	set exporter.Settings,
	cfg component.Config,
) (exporter.Metrics, error) {
	pcfg := cfg.(*Config)
	if err := pcfg.Validate(); err != nil {
		return nil, err
	}
	if !metadata.ExporterPrometheusexporterResourceConstantLabelsFeatureGate.IsEnabled() && pcfg.resourceToTelemetryConfigured() {
		set.Logger.Warn("`resource_to_telemetry_conversion` is deprecated. Please enable the `exporter.prometheusexporter.ResourceConstantLabels` feature gate and use `resource_constant_labels` instead.")
	}

	prometheus, err := newPrometheusExporter(pcfg, set)
	if err != nil {
		return nil, err
	}

	exporter, err := exporterhelper.NewMetrics(
		ctx,
		set,
		cfg,
		prometheus.ConsumeMetrics,
		exporterhelper.WithStart(prometheus.Start),
		exporterhelper.WithShutdown(prometheus.Shutdown),
		exporterhelper.WithQueue(pcfg.QueueBatchConfig),
		exporterhelper.WithCapabilities(consumer.Capabilities{MutatesData: true}),
	)
	if err != nil {
		return nil, err
	}
	metricsExporter := exporter
	if !metadata.ExporterPrometheusexporterResourceConstantLabelsFeatureGate.IsEnabled() {
		metricsExporter = resourcetotelemetry.WrapMetricsExporter(pcfg.ResourceToTelemetrySettings, exporter)
	}

	return &wrapMetricsExporter{
		Metrics:  metricsExporter,
		exporter: prometheus,
	}, nil
}

type wrapMetricsExporter struct {
	exporter.Metrics
	exporter *prometheusExporter
}
