// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlesecopsexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/googlesecopsexporter"

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

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
	defaultBatchRequestSizeLimit = 4000000
	defaultMetricsInterval       = 1 * time.Minute
	defaultCollectorID           = "aaaa1111-aaaa-1111-aaaa-1111aaaa1111"
)

// createDefaultConfig creates the default configuration for the google secops exporter.
func createDefaultConfig() component.Config {
	return &Config{
		APIVersion:            apiVersionV1Alpha,
		LogErroredPayloads:    false,
		ValidateLogTypes:      false,
		CollectAgentMetrics:   true,
		MetricsInterval:       defaultMetricsInterval,
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
	t, err := metadata.NewTelemetryBuilder(params.TelemetrySettings)
	if err != nil {
		return nil, fmt.Errorf("create telemetry builder: %w", err)
	}

	c := cfg.(*Config)
	if c.API == chronicleAPI {
		exp, err = newChronicleAPIExporter(c, params, t)
	} else {
		exp, err = newBackstoryAPIExporter(c, params, t)
	}
	if err != nil {
		return nil, err
	}
	return exporterhelper.NewLogs(
		ctx,
		params,
		c,
		exp.ConsumeLogs,
		exporterhelper.WithCapabilities(exp.Capabilities()),
		exporterhelper.WithTimeout(c.TimeoutConfig),
		exporterhelper.WithQueue(c.QueueBatchConfig),
		exporterhelper.WithRetry(c.BackOffConfig),
		exporterhelper.WithStart(exp.Start),
		exporterhelper.WithShutdown(exp.Shutdown),
	)
}
