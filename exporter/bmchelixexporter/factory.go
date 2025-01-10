// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package bmchelixexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/bmchelixexporter"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/bmchelixexporter/internal/metadata"
)

// create BMC Helix Exporter factory
func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		metadata.Type,
		createDefaultConfig,
		exporter.WithMetrics(createMetricsExporter, metadata.MetricsStability),
	)
}

// creates the default configuration for the BMC Helix exporter
func createDefaultConfig() component.Config {
	return &Config{
		Timeout:     10 * time.Second,
		RetryConfig: configretry.NewDefaultBackOffConfig(),
	}
}

// creates an exporter.Metrics that records observability metrics for BMC Helix
func createMetricsExporter(ctx context.Context, set exporter.Settings, cfg component.Config) (exporter.Metrics, error) {
	return exporterhelper.NewMetrics(
		ctx,
		set,
		cfg,
		func(_ context.Context, _ pmetric.Metrics) error {
			return nil
		},
	)
}
