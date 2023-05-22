// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusexporter"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
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
		ConstLabels:       map[string]string{},
		SendTimestamps:    false,
		MetricExpiration:  time.Minute * 5,
		EnableOpenMetrics: false,
	}
}

func createMetricsExporter(
	ctx context.Context,
	set exporter.CreateSettings,
	cfg component.Config,
) (exporter.Metrics, error) {
	pcfg := cfg.(*Config)

	prometheus, err := newPrometheusExporter(pcfg, set)
	if err != nil {
		return nil, err
	}

	exporter, err := exporterhelper.NewMetricsExporter(
		ctx,
		set,
		cfg,
		prometheus.ConsumeMetrics,
		exporterhelper.WithStart(prometheus.Start),
		exporterhelper.WithShutdown(prometheus.Shutdown),
	)
	if err != nil {
		return nil, err
	}

	return &wrapMetricsExporter{
		Metrics:  resourcetotelemetry.WrapMetricsExporter(pcfg.ResourceToTelemetrySettings, exporter),
		exporter: prometheus,
	}, nil
}

type wrapMetricsExporter struct {
	exporter.Metrics
	exporter *prometheusExporter
}
