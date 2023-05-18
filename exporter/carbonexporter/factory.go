// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package carbonexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/carbonexporter"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter"
)

const (
	// The value of "type" key in configuration.
	typeStr = "carbon"
	// The stability level of the exporter.
	stability = component.StabilityLevelBeta
)

// NewFactory creates a factory for Carbon exporter.
func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		typeStr,
		createDefaultConfig,
		exporter.WithMetrics(createMetricsExporter, stability))
}

func createDefaultConfig() component.Config {
	return &Config{
		Endpoint: DefaultEndpoint,
		Timeout:  DefaultSendTimeout,
	}
}

func createMetricsExporter(
	_ context.Context,
	params exporter.CreateSettings,
	config component.Config,
) (exporter.Metrics, error) {
	exp, err := newCarbonExporter(config.(*Config), params)

	if err != nil {
		return nil, err
	}

	return exp, nil
}
