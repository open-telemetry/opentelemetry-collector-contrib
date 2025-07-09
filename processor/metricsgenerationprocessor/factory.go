// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metricsgenerationprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricsgenerationprocessor"

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processorhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricsgenerationprocessor/internal/metadata"
)

var processorCapabilities = consumer.Capabilities{MutatesData: true}

// NewFactory returns a new factory for the Metrics Generation processor.
func NewFactory() processor.Factory {
	return processor.NewFactory(
		metadata.Type,
		createDefaultConfig,
		processor.WithMetrics(createMetricsProcessor, metadata.MetricsStability))
}

func createDefaultConfig() component.Config {
	return &Config{}
}

func createMetricsProcessor(
	ctx context.Context,
	set processor.Settings,
	cfg component.Config,
	nextConsumer consumer.Metrics,
) (processor.Metrics, error) {
	processorConfig, ok := cfg.(*Config)
	if !ok {
		return nil, errors.New("configuration parsing error")
	}

	metricsProcessor := newMetricsGenerationProcessor(buildInternalConfig(processorConfig), set.Logger)

	return processorhelper.NewMetrics(
		ctx,
		set,
		cfg,
		nextConsumer,
		metricsProcessor.processMetrics,
		processorhelper.WithCapabilities(processorCapabilities))
}

// buildInternalConfig constructs the internal metric generation rules
func buildInternalConfig(config *Config) []internalRule {
	internalRules := make([]internalRule, len(config.Rules))

	for i, rule := range config.Rules {
		customRule := internalRule{
			name:      rule.Name,
			unit:      rule.Unit,
			ruleType:  string(rule.Type),
			metric1:   rule.Metric1,
			metric2:   rule.Metric2,
			operation: string(rule.Operation),
			scaleBy:   rule.ScaleBy,
		}
		internalRules[i] = customRule
	}
	return internalRules
}
