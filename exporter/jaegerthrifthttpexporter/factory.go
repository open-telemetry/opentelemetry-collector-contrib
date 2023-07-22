// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package jaegerthrifthttpexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/jaegerthrifthttpexporter"

import (
	"context"
	"sync"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/jaegerthrifthttpexporter/internal/metadata"
)

var once sync.Once

// NewFactory creates a factory for Jaeger Thrift over HTTP exporter.
func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		metadata.Type,
		createDefaultConfig,
		exporter.WithTraces(createTracesExporter, metadata.TracesStability))
}

func createDefaultConfig() component.Config {
	return &Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Timeout: exporterhelper.NewDefaultTimeoutSettings().Timeout,
		},
	}
}

func logDeprecation(logger *zap.Logger) {
	once.Do(func() {
		logger.Warn("jaeger_thrift exporter is deprecated and will be removed in July 2023. See https://github.com/open-telemetry/opentelemetry-specification/pull/2858 for more details.")
	})
}

func createTracesExporter(
	_ context.Context,
	set exporter.CreateSettings,
	config component.Config,
) (exporter.Traces, error) {
	logDeprecation(set.Logger)
	expCfg := config.(*Config)
	return newTracesExporter(expCfg, set)
}
