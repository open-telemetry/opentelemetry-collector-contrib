// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

package sematextexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sematextexporter"

import (
	"context"
	"time"

	"github.com/influxdata/influxdb-observability/common"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sematextexporter/internal/metadata"
)

const (
	metricsAppToken = "2064e37c-4fac-45f6-831d-922d43fde759"
	logsAppToken    = "9064e37c-4gac-49f6-831d-922l43fse759"
)

// NewFactory creates a factory for the Sematext metrics exporter.
func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		metadata.Type,
		createDefaultConfig,
		exporter.WithMetrics(createMetricsExporter, metadata.MetricsStability),
		exporter.WithLogs(createLogsExporter, metadata.LogsStability),
	)
}

func createDefaultConfig() component.Config {
	cfg := &Config{
		ClientConfig: confighttp.ClientConfig{
			Timeout: 5 * time.Second,
			Headers: map[string]configopaque.String{
				"User-Agent": "OpenTelemetry -> Sematext",
			},
		},
		MetricsConfig: MetricsConfig{
			MetricsEndpoint: usMetricsEndpoint,
			MetricsSchema:   common.MetricsSchemaTelegrafPrometheusV2.String(),
			AppToken:        metricsAppToken,
			QueueSettings:   exporterhelper.NewDefaultQueueConfig(),
			PayloadMaxLines: 1_000,
			PayloadMaxBytes: 300_000,
		},
		LogsConfig: LogsConfig{
			LogsEndpoint: usLogsEndpoint,
			AppToken:     logsAppToken,
		},
		BackOffConfig: configretry.NewDefaultBackOffConfig(),
		Region:        usRegion,
	}
	return cfg
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

func createLogsExporter(
	ctx context.Context,
	set exporter.Settings,
	config component.Config,
) (exporter.Logs, error) {
	cfg := config.(*Config)

	return exporterhelper.NewLogs(
		ctx,
		set,
		cfg,
		func(_ context.Context, _ plog.Logs) error {
			return nil
		},
	)
}
