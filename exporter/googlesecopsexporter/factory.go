// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlesecopsexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/googlesecopsexporter"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/googlesecopsexporter/internal/metadata"
)

// NewFactory creates a new Google SecOps exporter factory.
func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		metadata.Type,
		createDefaultConfig,
		exporter.WithLogs(createLogsExporter, metadata.LogsStability))
}

const (
	defaultHostname              = "chronicle.googleapis.com"
	defaultBatchRequestSizeLimit = 4000000
	defaultMetricsInterval       = 1 * time.Minute
	defaultCollectorID           = "aaaa1111-aaaa-1111-aaaa-1111aaaa1111"
)

// createDefaultConfig creates the default configuration for the google secops exporter.
func createDefaultConfig() component.Config {
	return &Config{
		API:                   chronicleAPI,
		Hostname:              defaultHostname,
		CollectAgentMetrics:   true,
		MetricsInterval:       defaultMetricsInterval,
		LogErroredPayloads:    false,
		ValidateLogTypes:      false,
		Compression:           noCompression,
		BatchRequestSizeLimit: defaultBatchRequestSizeLimit,
		TimeoutConfig:         exporterhelper.NewDefaultTimeoutConfig(),
		QueueBatchConfig:      configoptional.Some(exporterhelper.NewDefaultQueueConfig()),
		BackOffConfig:         configretry.NewDefaultBackOffConfig(),
		CollectorID:           defaultCollectorID,
	}
}

// createLogsExporter creates a new log exporter based on this config.
func createLogsExporter(
	ctx context.Context,
	params exporter.Settings,
	cfg component.Config,
) (exp exporter.Logs, err error) {
	c := cfg.(*Config)
	return exporterhelper.NewLogs(
		ctx,
		params,
		c,
		// TODO: Replace no-op consumer with actual exporter implementation.
		func(_ context.Context, _ plog.Logs) error {
			return nil
		},
		exporterhelper.WithTimeout(c.TimeoutConfig),
		exporterhelper.WithQueue(c.QueueBatchConfig),
		exporterhelper.WithRetry(c.BackOffConfig),
	)
}
