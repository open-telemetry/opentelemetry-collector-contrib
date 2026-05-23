// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate make mdatagen

package opensearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opensearchexporter"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opensearchexporter/internal/metadata"
)

// NewFactory creates a factory for OpenSearch exporter.
func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		metadata.Type,
		newDefaultConfig,
		exporter.WithTraces(createTracesExporter, metadata.TracesStability),
		exporter.WithLogs(createLogsExporter, metadata.LogsStability),
	)
}

func newDefaultConfig() component.Config {
	return &Config{
		ClientConfig:     confighttp.NewDefaultClientConfig(),
		Dataset:          defaultDataset,
		Namespace:        defaultNamespace,
		BulkAction:       defaultBulkAction,
		BackOffConfig:    configretry.NewDefaultBackOffConfig(),
		MappingsSettings: MappingsSettings{Mode: defaultMappingMode},
		QueueConfig:      configoptional.Default(exporterhelper.NewDefaultQueueConfig()),
	}
}

func createTracesExporter(ctx context.Context,
	set exporter.Settings,
	cfg component.Config,
) (exporter.Traces, error) {
	c := cfg.(*Config)
	te := newSSOTracesExporter(c, set)

	return exporterhelper.NewTraces(ctx, set, cfg,
		te.pushTraceData,
		exporterhelper.WithStart(te.Start),
		exporterhelper.WithCapabilities(consumer.Capabilities{MutatesData: false}),
		exporterhelper.WithRetry(c.BackOffConfig),
		exporterhelper.WithQueue(c.QueueConfig),
		exporterhelper.WithTimeout(c.TimeoutSettings))
}

func createLogsExporter(ctx context.Context,
	set exporter.Settings,
	cfg component.Config,
) (exporter.Logs, error) {
	c := cfg.(*Config)
	le := newLogExporter(c, set)

	return exporterhelper.NewLogs(ctx, set, cfg,
		le.pushLogData,
		exporterhelper.WithStart(le.Start),
		exporterhelper.WithCapabilities(consumer.Capabilities{MutatesData: true}),
		exporterhelper.WithRetry(c.BackOffConfig),
		exporterhelper.WithQueue(c.QueueConfig),
		exporterhelper.WithTimeout(c.TimeoutSettings))
}
