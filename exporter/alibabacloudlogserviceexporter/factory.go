// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package alibabacloudlogserviceexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/alibabacloudlogserviceexporter"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
)

const (
	// The value of "type" key in configuration.
	typeStr = "alibabacloud_logservice"
	// The stability level of the exporter.
	stability = component.StabilityLevelBeta
)

// NewFactory creates a factory for AlibabaCloud LogService exporter.
func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		typeStr,
		createDefaultConfig,
		exporter.WithTraces(createTracesExporter, stability),
		exporter.WithMetrics(createMetricsExporter, stability),
		exporter.WithLogs(createLogsExporter, stability))
}

// CreateDefaultConfig creates the default configuration for exporter.
func createDefaultConfig() component.Config {
	return &Config{}
}

func createTracesExporter(
	_ context.Context,
	set exporter.CreateSettings,
	cfg component.Config,
) (exporter.Traces, error) {
	return newTracesExporter(set, cfg)
}

func createMetricsExporter(
	_ context.Context,
	set exporter.CreateSettings,
	cfg component.Config,
) (exp exporter.Metrics, err error) {
	return newMetricsExporter(set, cfg)
}

func createLogsExporter(
	_ context.Context,
	set exporter.CreateSettings,
	cfg component.Config,
) (exp exporter.Logs, err error) {
	return newLogsExporter(set, cfg)
}
