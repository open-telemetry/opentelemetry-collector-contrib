// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package cardinalityguardianprocessor is documented in doc.go.
package cardinalityguardianprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/cardinalityguardianprocessor"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
)

// typeStr is the name operators use to reference this processor in pipeline
// configuration (e.g. `processors: [cardinality_guardian]`).
const typeStr = "cardinality_guardian"

// NewFactory creates the cardinality_guardian processor factory. It is
// registered at StabilityLevelDevelopment — the API and configuration schema
// may change between releases.
func NewFactory() processor.Factory {
	return processor.NewFactory(
		component.MustNewType(typeStr),
		createDefaultConfig,
		processor.WithMetrics(createMetricsProcessor, component.StabilityLevelDevelopment),
	)
}

// createDefaultConfig returns a *Config with conservative defaults suitable
// for most production environments. See the Config field docs for the
// meaning of each setting.
func createDefaultConfig() component.Config {
	return &Config{
		MaxCardinalityDeltaPerEpoch: 100,
		EpochDurationSeconds:        300,
		NeverDropLabels:             []string{"http.status_code", "region"},
		EnforcementMode:             EnforcementTagOnly,
		EstimatedCostPerMetricMonth: 0.05, // Default to $0.05 per metric/month
		TopOffendersCount:           10,   // Report the top 10 exploding (metric, label) pairs
		MaxTrackerCount:             0,    // Default to unlimited
		MetricOverrides:             nil,  // No per-metric overrides by default
		DropLogMaxPerEpoch:          10,   // Only log the first 10 drops per epoch
	}
}

// createMetricsProcessor is the constructor wired into the factory by
// processor.WithMetrics. The Collector calls it once per pipeline that
// references this component, passing the merged (default + user) configuration
// as a component.Config interface. The function performs a type assertion to
// *Config and delegates to the package-internal newCardinalityProcessor.
func createMetricsProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (processor.Metrics, error) {
	oCfg, ok := cfg.(*Config)
	if !ok {
		return nil, fmt.Errorf("invalid config type: expected *Config, got %T", cfg)
	}
	return newCardinalityProcessor(ctx, oCfg, set, nextConsumer)
}
