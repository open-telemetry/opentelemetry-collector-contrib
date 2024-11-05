// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

package sematextexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sematextexporter"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/influxdata/influxdb-observability/common"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sematextexporter/internal/metadata"
)

const appToken string = "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"

// NewFactory creates a factory for the Sematext metrics exporter.
func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		metadata.Type,
		createDefaultConfig,
		exporter.WithMetrics(createMetricsExporter, metadata.MetricsStability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		ClientConfig: confighttp.ClientConfig{
			Timeout: 5 * time.Second,
			Headers: map[string]configopaque.String{
				"User-Agent": "OpenTelemetry -> Sematext",
			},
		},
		MetricsConfig: MetricsConfig{
			MetricsSchema:   common.MetricsSchemaTelegrafPrometheusV2.String(),
			AppToken:        appToken,
			QueueSettings:   exporterhelper.NewDefaultQueueConfig(),
			PayloadMaxLines: 1_000,
			PayloadMaxBytes: 300_000,
		},
		BackOffConfig: configretry.NewDefaultBackOffConfig(),
		Region:        "custom",
	}
}

func createMetricsExporter(
	ctx context.Context,
	set exporter.Settings,
	config component.Config,
) (exporter.Metrics, error) {
	cfg := config.(*Config)

	return exporterhelper.NewMetrics(
		ctx,
		set,
		cfg,
		func(_ context.Context, _ pmetric.Metrics) error {
			return nil
		},
	)
}
