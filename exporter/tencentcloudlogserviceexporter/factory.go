// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:generate make mdatagen

package tencentcloudlogserviceexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/tencentcloudlogserviceexporter"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/xexporter"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/tencentcloudlogserviceexporter/internal/metadata"
)

// NewFactory creates a factory for tencentcloud LogService exporter.
func NewFactory() exporter.Factory {
	return xexporter.NewFactory(
		metadata.Type,
		createDefaultConfig,
		xexporter.WithLogs(createLogsExporter, metadata.LogsStability),
		xexporter.WithDeprecatedTypeAlias(metadata.DeprecatedType),
	)
}

// CreateDefaultConfig creates the default configuration for exporter.
func createDefaultConfig() component.Config {
	return &Config{}
}

func createLogsExporter(
	_ context.Context,
	set exporter.Settings,
	cfg component.Config,
) (exp exporter.Logs, err error) {
	return newLogsExporter(set, cfg)
}
