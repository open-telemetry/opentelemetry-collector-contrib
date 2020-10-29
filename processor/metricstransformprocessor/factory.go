// Copyright 2020 OpenTelemetry Authors
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

package metricstransformprocessor

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor/processorhelper"
)

const (
	// The value of "type" key in configuration.
	typeStr = "metricstransform"
)

var processorCapabilities = component.ProcessorCapabilities{MutatesConsumedData: true}

// NewFactory returns a new factory for the Metrics Transform processor.
func NewFactory() component.ProcessorFactory {
	return processorhelper.NewFactory(
		typeStr,
		createDefaultConfig,
		processorhelper.WithMetrics(createMetricsProcessor))
}

func createDefaultConfig() configmodels.Processor {
	return &Config{
		ProcessorSettings: configmodels.ProcessorSettings{
			TypeVal: typeStr,
			NameVal: typeStr,
		},
	}
}

func createMetricsProcessor(
	ctx context.Context,
	params component.ProcessorCreateParams,
	cfg configmodels.Processor,
	nextConsumer consumer.MetricsConsumer,
) (component.MetricsProcessor, error) {
	oCfg := cfg.(*Config)
	if err := validateConfiguration(oCfg); err != nil {
		return nil, err
	}

	internalTransforms, err := buildHelperConfig(oCfg, params.ApplicationStartInfo.Version)
	if err != nil {
		return nil, err
	}

	metricsProcessor := newMetricsTransformProcessor(params.Logger, internalTransforms)

	return processorhelper.NewMetricsProcessor(
		cfg,
		nextConsumer,
		metricsProcessor,
		processorhelper.WithCapabilities(processorCapabilities))
}

// validateConfiguration validates the input configuration has all of the required fields for the processor
// An error is returned if there are any invalid inputs.
func validateConfiguration(config *Config) error {
	for _, transform := range config.Transforms {
		if transform.MetricIncludeFilter.Include == "" && transform.MetricName == "" {
			return fmt.Errorf("missing required field %q", IncludeFieldName)
		}

		if transform.MetricIncludeFilter.MatchType != "" {
			var validMatchType bool
			for _, matchType := range MatchTypes {
				if transform.MetricIncludeFilter.MatchType == matchType {
					validMatchType = true
					break
				}
			}

			if !validMatchType {
				return fmt.Errorf("%q must be in %q", MatchTypeFieldName, MatchTypes)
			}
		}

		if transform.MetricIncludeFilter.Include != "" && transform.MetricName != "" {
			return fmt.Errorf("cannot supply both %q and %q, use %q with %q match type", IncludeFieldName, MetricNameFieldName, IncludeFieldName, StrictMatchType)
		}

		if transform.Action != Update && transform.Action != Insert {
			return fmt.Errorf("unsupported %q: %v, the supported actions are %q and %q", ActionFieldName, transform.Action, Insert, Update)
		}

		if transform.Action == Insert && transform.NewName == "" {
			return fmt.Errorf("missing required field %q while %q is %v", NewNameFieldName, ActionFieldName, Insert)
		}

		for i, op := range transform.Operations {
			if op.Action == UpdateLabel && op.Label == "" {
				return fmt.Errorf("missing required field %q while %q is %v in the %vth operation", LabelFieldName, ActionFieldName, UpdateLabel, i)
			}
			if op.Action == AddLabel && op.NewLabel == "" {
				return fmt.Errorf("missing required field %q while %q is %v in the %vth operation", NewLabelFieldName, ActionFieldName, AddLabel, i)
			}
			if op.Action == AddLabel && op.NewValue == "" {
				return fmt.Errorf("missing required field %q while %q is %v in the %vth operation", NewValueFieldName, ActionFieldName, AddLabel, i)
			}
		}
	}
	return nil
}

// buildHelperConfig constructs the maps that will be useful for the operations
func buildHelperConfig(config *Config, version string) ([]internalTransform, error) {
	helperDataTransforms := make([]internalTransform, len(config.Transforms))
	for i, t := range config.Transforms {
		// for backwards compatibility, convert metric name to an include filter
		if t.MetricName != "" {
			t.MetricIncludeFilter = FilterConfig{
				Include:   t.MetricName,
				MatchType: StrictMatchType,
			}
		}

		filter, err := createFilter(t.MetricIncludeFilter)
		if err != nil {
			return nil, err
		}

		helperT := internalTransform{
			MetricIncludeFilter: filter,
			Action:              t.Action,
			NewName:             t.NewName,
			Operations:          make([]internalOperation, len(t.Operations)),
		}

		for j, op := range t.Operations {
			op.NewValue = strings.ReplaceAll(op.NewValue, "{{version}}", version)

			mtpOp := internalOperation{
				configOperation: op,
			}
			if len(op.ValueActions) > 0 {
				mtpOp.valueActionsMapping = createLabelValueMapping(op.ValueActions, version)
			}
			if op.Action == AggregateLabels {
				mtpOp.labelSetMap = sliceToSet(op.LabelSet)
			} else if op.Action == AggregateLabelValues {
				mtpOp.aggregatedValuesSet = sliceToSet(op.AggregatedValues)
			}
			helperT.Operations[j] = mtpOp
		}
		helperDataTransforms[i] = helperT
	}
	return helperDataTransforms, nil
}

func createFilter(filterConfig FilterConfig) (internalFilter, error) {
	switch filterConfig.MatchType {
	case RegexpMatchType:
		r, err := regexp.Compile(filterConfig.Include)
		if err != nil {
			return nil, err
		}
		return internalFilterRegexp{include: r}, nil
	default:
		return internalFilterStrict{include: filterConfig.Include}, nil
	}
}

// createLabelValueMapping creates the labelValue rename mappings based on the valueActions
func createLabelValueMapping(valueActions []ValueAction, version string) map[string]string {
	mapping := make(map[string]string)
	for i := 0; i < len(valueActions); i++ {
		valueActions[i].NewValue = strings.ReplaceAll(valueActions[i].NewValue, "{{version}}", version)
		mapping[valueActions[i].Value] = valueActions[i].NewValue
	}
	return mapping
}

// sliceToSet converts slice of strings to set of strings
// Returns the set of strings
func sliceToSet(slice []string) map[string]bool {
	set := make(map[string]bool, len(slice))
	for _, s := range slice {
		set[s] = true
	}
	return set
}
