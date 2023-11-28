// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package honeycombmarkerexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/honeycombmarkerexporter"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/honeycombmarkerexporter/internal/metadata"
)

func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		metadata.Type,
		createDefaultConfig,
		exporter.WithLogs(createLogsExporter, metadata.LogsStability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		APIKey:  "",
		APIURL:  "api.honeycomb.io:443",
		Markers: []Marker{},
	}
}

func createLogsExporter(
	ctx context.Context,
	set exporter.CreateSettings,
	cfg component.Config,
) (exporter.Logs, error) {
	cf := cfg.(*Config)

	exporter := newLogsExporter(set.Logger, cf)

	return exporterhelper.NewLogsExporter(
		ctx,
		set,
		cfg,
		exporter.exportLogs,
	)
}
