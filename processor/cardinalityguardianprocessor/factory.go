// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package cardinalityguardianprocessor is documented in processor.go.
package cardinalityguardianprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/cardinalityguardianprocessor"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
)

// typeStr is the canonical component type identifier for the cardinality_guardian
// processor. It is used in three places:
//
//  1. processor.NewFactory — registers the component under this name so the
//     Collector can find it when parsing the pipeline YAML.
//  2. newCardinalityProcessor — passed to MeterProvider.Meter() so that every
//     OTel metric emitted by this component carries a consistent instrumentation
//     scope name, making it straightforward to filter dashboards or alert rules
//     by component.
//  3. processortest.NewNopSettings — test helpers use component.MustNewType(typeStr)
//     so the mock settings carry the same type attribute as production.
const typeStr = "cardinality_guardian"

// NewFactory creates and registers the cardinality_guardian processor factory
// with the OpenTelemetry Collector. The factory is what the Collector calls
// when it reads a pipeline configuration; it is responsible for providing a
// default config and a constructor for the actual processor component.
//
// The component is registered at StabilityLevelDevelopment, which signals to
// operators that the API and configuration schema may change between releases.
// Promote to StabilityLevelBeta or StabilityLevelStable once the configuration
// schema is considered stable.
func NewFactory() processor.Factory {
	return processor.NewFactory(
		component.MustNewType(typeStr),
		createDefaultConfig,
		processor.WithMetrics(createMetricsProcessor, component.StabilityLevelDevelopment),
	)
}

// createDefaultConfig returns a *Config populated with safe, conservative
// defaults that are suitable for most production environments:
//
//   - MaxCardinalityDeltaPerEpoch: 100 — allows up to 100 new unique label values
//     per metric+key per 5-minute window before enforcement kicks in. This accommodates
//     common labels while still catching runaway explosions.
//   - EpochDurationSeconds: 300 — a 5-minute sliding window that aligns well
//     with common scrape intervals and on-call alert evaluation periods.
//   - NeverDropLabels: "http.status_code" and "region" — two labels that are
//     almost universally required for meaningful query results.
//   - TagOnly: false — hard-drop mode by default, which is the safest option
//     for operators who want to immediately reduce ingestion costs.
//   - EstimatedCostPerMetricMonth: $0.05 — a reasonable approximation of the
//     per-series cost for managed Prometheus services.
func createDefaultConfig() component.Config {
	return &Config{
		MaxCardinalityDeltaPerEpoch: 100,
		EpochDurationSeconds:        300,
		NeverDropLabels:             []string{"http.status_code", "region"},
		TagOnly:                     false, // Default to hard-drop for backward compatibility
		EstimatedCostPerMetricMonth: 0.05,  // Default to $0.05 per metric/month
		TopOffendersCount:           10,    // Report the top 10 exploding (metric, label) pairs
		MaxTrackerCount:             0,     // Default to unlimited
		MetricOverrides:             nil,   // No per-metric overrides by default
		DropLogMaxPerEpoch:          10,    // Only log the first 10 drops per epoch
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
