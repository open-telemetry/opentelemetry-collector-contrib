// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudstorageexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/googlecloudstorageexporter"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/xexporter"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/googlecloudstorageexporter/internal/metadata"
)

func NewFactory() exporter.Factory {
	return xexporter.NewFactory(
		metadata.Type,
		createDefaultConfig,
		xexporter.WithLogs(createLogsExporter, metadata.LogsStability),
		xexporter.WithTraces(createTracesExporter, metadata.TracesStability),
		xexporter.WithDeprecatedTypeAlias(metadata.DeprecatedType),
	)
}

func createLogsExporter(ctx context.Context, set exporter.Settings, config component.Config) (exporter.Logs, error) {
	return newGCSExporter(ctx, config.(*Config), set.Logger, signalTypeLogs)
}

func createTracesExporter(ctx context.Context, set exporter.Settings, config component.Config) (exporter.Traces, error) {
	return newGCSExporter(ctx, config.(*Config), set.Logger, signalTypeTraces)
}
