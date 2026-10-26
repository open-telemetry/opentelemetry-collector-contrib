// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package clickhouseexporter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter/internal/metrics"
)

func Test_generateMetricMetricTableNames(t *testing.T) {
	cfg := Config{
		MetricsTables: MetricTablesConfig{
			Gauge:                metrics.MetricTypeConfig{Name: "otel_metrics_custom_gauge"},
			Sum:                  metrics.MetricTypeConfig{Name: "otel_metrics_custom_sum"},
			Summary:              metrics.MetricTypeConfig{Name: "otel_metrics_custom_summary"},
			Histogram:            metrics.MetricTypeConfig{Name: "otel_metrics_custom_histogram"},
			ExponentialHistogram: metrics.MetricTypeConfig{Name: "otel_metrics_custom_exp_histogram"},
		},
	}

	require.Equal(t, metrics.MetricTablesConfigMapper{
		pmetric.MetricTypeGauge:                cfg.MetricsTables.Gauge,
		pmetric.MetricTypeSum:                  cfg.MetricsTables.Sum,
		pmetric.MetricTypeSummary:              cfg.MetricsTables.Summary,
		pmetric.MetricTypeHistogram:            cfg.MetricsTables.Histogram,
		pmetric.MetricTypeExponentialHistogram: cfg.MetricsTables.ExponentialHistogram,
	}, generateMetricTablesConfigMapper(&cfg))
}

func TestMetricsExporterConnectionPoolSize(t *testing.T) {
	tests := []struct {
		name             string
		endpoint         string
		connectionParams map[string]string
		queue            configoptional.Optional[exporterhelper.QueueBatchConfig]
		expectedMaxOpen  int
		expectedMaxIdle  int
	}{
		{
			name:            "default queue",
			endpoint:        defaultEndpoint,
			queue:           configoptional.Some(exporterhelper.NewDefaultQueueConfig()),
			expectedMaxOpen: 50,
			expectedMaxIdle: 50,
		},
		{
			// Falls back to the driver defaults.
			name:            "queue disabled",
			endpoint:        defaultEndpoint,
			queue:           configoptional.None[exporterhelper.QueueBatchConfig](),
			expectedMaxOpen: 10,
			expectedMaxIdle: 5,
		},
		{
			name:            "endpoint overrides max open",
			endpoint:        defaultEndpoint + "?max_open_conns=3",
			queue:           configoptional.Some(exporterhelper.NewDefaultQueueConfig()),
			expectedMaxOpen: 3,
			expectedMaxIdle: 3,
		},
		{
			name:            "endpoint overrides max idle",
			endpoint:        defaultEndpoint + "?max_idle_conns=2",
			queue:           configoptional.Some(exporterhelper.NewDefaultQueueConfig()),
			expectedMaxOpen: 50,
			expectedMaxIdle: 2,
		},
		{
			name:             "connection params override",
			endpoint:         defaultEndpoint,
			connectionParams: map[string]string{"max_open_conns": "7"},
			queue:            configoptional.Some(exporterhelper.NewDefaultQueueConfig()),
			expectedMaxOpen:  7,
			expectedMaxIdle:  7,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)
			cfg.Endpoint = tt.endpoint
			cfg.QueueSettings = tt.queue
			if tt.connectionParams != nil {
				cfg.ConnectionParams = tt.connectionParams
			}
			// Skip schema creation so start doesn't need a ClickHouse server.
			cfg.CreateSchema = false

			exporter := newMetricsExporter(zap.NewNop(), cfg)
			require.NoError(t, exporter.start(context.Background(), nil))
			defer func() {
				require.NoError(t, exporter.shutdown(context.Background()))
			}()

			stats := exporter.db.Stats()
			assert.Equal(t, tt.expectedMaxOpen, stats.MaxOpenConns)
			assert.Equal(t, tt.expectedMaxIdle, stats.MaxIdleConns)
		})
	}
}
