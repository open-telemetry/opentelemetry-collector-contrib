// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package opsrampmetricsfilterprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/opsrampmetricsfilterprocessor"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
)

const (
	// The value of "type" key in configuration.
	typeStr = "opsrampmetricsfilter"
	// The stability level of the processor.
	stability = component.StabilityLevelAlpha
)

// NewFactory returns a new factory for the Alert Metrics Extractor processor.
func NewFactory() processor.Factory {
	return processor.NewFactory(
		component.MustNewType(typeStr),
		createDefaultConfig,
		processor.WithMetrics(createMetricsProcessor, stability),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		AlertConfigMapName: "opsramp-alert-user-config",
		AlertConfigMapKey:  "alert-definitions.yaml",
		Namespace:          "opsramp-agent",
	}
}

func createMetricsProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (processor.Metrics, error) {
	oCfg := cfg.(*Config)

	// Validate and set defaults
	if err := oCfg.Validate(); err != nil {
		return nil, err
	}

	return newFilterProcessor(set, oCfg, nextConsumer)
}
