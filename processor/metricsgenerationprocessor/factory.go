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
	typeStr = "metricsgeneration"
)

var processorCapabilities = component.ProcessorCapabilities{MutatesConsumedData: true}

// NewFactory returns a new factory for the Metrics Generation processor.
func NewFactory() component.ProcessorFactory {
	return processorhelper.NewFactory(
		typeStr,
		createDefaultConfig,
		processorhelper.WithMetrics(createMetricsProcessor))
}

func createDefaultConfig() config.Processor {
	return &Config{
		ProcessorSettings: config.NewProcessorSettings(typeStr),
	}
}

func createMetricsProcessor(
	ctx context.Context,
	params component.ProcessorCreateParams,
	cfg config.Processor,
	nextConsumer consumer.Metrics,
) (component.MetricsProcessor, error) {
	processorConfig := cfg.(*Config)
	if err := validateConfiguration(processorConfig); err != nil {
		return nil, err
	}

	metricsProcessor := newMetricsGenerationProcessor(buildInternalConfig(processorConfig), params.Logger)

	return processorhelper.NewMetricsProcessor(
		cfg,
		nextConsumer,
		metricsProcessor,
		processorhelper.WithCapabilities(processorCapabilities))
}

// validateConfiguration validates the input configuration has all of the required fields for the processor
// An error is returned if there are any invalid inputs.
func validateConfiguration(config *Config) error {
	for _, rule := range config.Rules {
		if rule.NewMetricName == "" {
			return fmt.Errorf("missing required field %q", NewMetricFieldName)
		}

		if rule.Type == "" {
			return fmt.Errorf("missing required field %q", GenerationTypeFieldName)
		}

		if !rule.Type.isValid() {
			return fmt.Errorf("%q must be in %q", GenerationTypeFieldName, generationTypes)
		}

		if rule.Operand1Metric == "" {
			return fmt.Errorf("missing required field %q", Operand1MetricFieldName)
		}

		if rule.Type == Calculate && rule.Operand2Metric == "" {
			return fmt.Errorf("missing required field %q for generation type %q", Operand2MetricFieldName, Calculate)
		}

		if rule.Type == Scale && rule.ScaleBy <= 0 {
			return fmt.Errorf("field %q required to be greater than 0 for generation type %q", ScaleByFieldName, Scale)
		}

		if rule.Operation != "" && !rule.Operation.isValid() {
			return fmt.Errorf("%q must be in %q", OperationFieldName, operationTypes)
		}
	}
	return nil
}

// buildInternalConfig constructs the internal metric genration rules
func buildInternalConfig(config *Config) []internalRule {
	internalRules := make([]internalRule, len(config.Rules))

	for i, rule := range config.Rules {
		internalRules[i] = internalRule(rule)
	}
	return internalRules
}
