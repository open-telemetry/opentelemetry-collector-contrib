// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opensearchexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opensearchexporter"

import (
	"context"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

const (
	// The value of "type" key in configuration.
	typeStr = "opensearch"
	// The stability level of the exporter.
	stability = component.StabilityLevelDevelopment
)

// NewFactory creates a factory for OpenSearch exporter.
func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		typeStr,
		newDefaultConfig,
		exporter.WithTraces(createTracesExporter, stability),
	)
}

func newDefaultConfig() component.Config {
	return &Config{
		HTTPClientSettings: confighttp.NewDefaultHTTPClientSettings(),
		Namespace:          "namespace",
		Dataset:            "default",
		RetrySettings:      exporterhelper.NewDefaultRetrySettings(),
	}
}

func createTracesExporter(ctx context.Context,
	set exporter.CreateSettings,
	cfg component.Config) (exporter.Traces, error) {

	return exporterhelper.NewTracesExporter(ctx, set, cfg, func(ctx context.Context, ld ptrace.Traces) error {
		return nil
	},
		exporterhelper.WithShutdown(func(ctx context.Context) error {
			return nil
		}))
}
