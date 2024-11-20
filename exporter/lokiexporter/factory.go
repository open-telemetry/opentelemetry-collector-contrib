// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate mdatagen metadata.yaml

package lokiexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/lokiexporter"

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/lokiexporter/internal/metadata"
)

// NewFactory creates a factory for the legacy Loki exporter.
func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		metadata.Type,
		createDefaultConfig,
		exporter.WithLogs(createLogsExporter, metadata.LogsStability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		ClientConfig: confighttp.ClientConfig{
			Endpoint: "",
			Timeout:  30 * time.Second,
			Headers:  map[string]configopaque.String{},
			// We almost read 0 bytes, so no need to tune ReadBufferSize.
			WriteBufferSize: 512 * 1024,
		},
		BackOffConfig: configretry.NewDefaultBackOffConfig(),
		QueueSettings: exporterhelper.NewDefaultQueueConfig(),
		DefaultLabelsEnabled: map[string]bool{
			"exporter": true,
			"job":      true,
			"instance": true,
			"level":    true,
		},
	}
}

func createLogsExporter(ctx context.Context, set exporter.Settings, config component.Config) (exporter.Logs, error) {
	exporterConfig := config.(*Config)
	exp, err := newExporter(exporterConfig, set.TelemetrySettings)
	if err != nil {
		return nil, err
	}

	return exporterhelper.NewLogs(
		ctx,
		set,
		config,
		exp.pushLogData,
		// explicitly disable since we rely on http.Client timeout logic.
		exporterhelper.WithTimeout(exporterhelper.TimeoutConfig{Timeout: 0}),
		exporterhelper.WithRetry(exporterConfig.BackOffConfig),
		exporterhelper.WithQueue(exporterConfig.QueueSettings),
		exporterhelper.WithStart(exp.start),
		exporterhelper.WithShutdown(exp.stop),
	)
}
