// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package metricsgenerationprocessor

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor/processorhelper"
)

const (
	// The value of "type" key in configuration.
	typeStr = "experimental_metricsgeneration"
)

var processorCapabilities = consumer.Capabilities{MutatesData: true}

// NewFactory returns a new factory for the Metrics Generation processor.
func NewFactory() component.ProcessorFactory {
	return processorhelper.NewFactory(
		typeStr,
		createDefaultConfig,
		processorhelper.WithMetrics(createMetricsProcessor))
}

func createDefaultConfig() config.Processor {
	return &Config{
		ProcessorSettings: config.NewProcessorSettings(config.NewID(typeStr)),
	}
}

func createMetricsProcessor(
	ctx context.Context,
	params component.ProcessorCreateSettings,
	cfg config.Processor,
	nextConsumer consumer.Metrics,
) (component.MetricsProcessor, error) {
	processorConfig, ok := cfg.(*Config)
	if !ok {
		return nil, fmt.Errorf("configuration parsing error")
	}

	processorConfig.Validate()
	metricsProcessor := newMetricsGenerationProcessor(buildInternalConfig(processorConfig), params.Logger)

	return processorhelper.NewMetricsProcessor(
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
