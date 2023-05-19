// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sentryexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sentryexporter"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
)

const (
	typeStr = "sentry"
	// The stability level of the exporter.
	stability = component.StabilityLevelBeta
)

// NewFactory creates a factory for Sentry exporter.
func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		typeStr,
		createDefaultConfig,
		exporter.WithTraces(createTracesExporter, stability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{}
}

func createTracesExporter(
	_ context.Context,
	params exporter.CreateSettings,
	config component.Config,
) (exporter.Traces, error) {
	sentryConfig, ok := config.(*Config)
	if !ok {
		return nil, fmt.Errorf("unexpected config type: %T", config)
	}

	// Create exporter based on sentry config.
	exp, err := CreateSentryExporter(sentryConfig, params)
	return exp, err
}
