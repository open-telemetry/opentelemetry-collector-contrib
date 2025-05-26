// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

package sematextexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sematextexporter"

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/influxdata/influxdb-observability/common"
	"github.com/influxdata/influxdb-observability/otel2influx"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sematextexporter/internal/metadata"
)

var (
	metricsAppToken = uuid.NewString()
	logsAppToken    = uuid.NewString()
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
		QueueSettings: exporterhelper.NewDefaultQueueConfig(),
		MetricsConfig: MetricsConfig{
			MetricsSchema:   common.MetricsSchemaTelegrafPrometheusV2.String(),
			PayloadMaxLines: 1_000,
			PayloadMaxBytes: 300_000,
		},
		BackOffConfig: configretry.NewDefaultBackOffConfig(),
	}
	return cfg
}

func createMetricsExporter(
	ctx context.Context,
	set exporter.Settings,
	config component.Config,
) (exporter.Metrics, error) {
	cfg := config.(*Config)

	// Initialize the logger for Sematext
	logger := newZapSematextLogger(set.Logger)

	// Create a writer for sending metrics to Sematext
	writer, err := newSematextHTTPWriter(logger, cfg, set.TelemetrySettings)
	if err != nil {
		return nil, fmt.Errorf("failed to create Sematext HTTP writer: %w", err)
	}
	schema, found := common.MetricsSchemata[cfg.MetricsSchema]
	if !found {
		return nil, fmt.Errorf("schema '%s' not recognized", cfg.MetricsSchema)
	}

	expConfig := otel2influx.DefaultOtelMetricsToLineProtocolConfig()
	expConfig.Logger = logger
	expConfig.Writer = writer
	expConfig.Schema = schema
	exp, err := otel2influx.NewOtelMetricsToLineProtocol(expConfig)
	if err != nil {
		return nil, err
	}

	return exporterhelper.NewMetrics(
		ctx,
		set,
		cfg,
		exp.WriteMetrics,
		exporterhelper.WithQueue(cfg.QueueSettings),
		exporterhelper.WithRetry(cfg.BackOffConfig),
		exporterhelper.WithStart(writer.Start),
		exporterhelper.WithShutdown(writer.Shutdown),
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
