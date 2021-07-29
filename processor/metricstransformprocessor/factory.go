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
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor/processorhelper"
)

const (
	// The value of "type" key in configuration.
	typeStr = "metricstransform"
)

var consumerCapabilities = consumer.Capabilities{MutatesData: true}

// NewFactory returns a new factory for the Metrics Transform processor.
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
	oCfg := cfg.(*Config)
	if err := validateConfiguration(oCfg); err != nil {
		return nil, err
	}

	hCfg, err := buildHelperConfig(oCfg, params.BuildInfo.Version)
	if err != nil {
		return nil, err
	}
	metricsProcessor := newMetricsTransformProcessor(params.Logger, hCfg)

	return processorhelper.NewMetricsProcessor(
		cfg,
		nextConsumer,
		metricsProcessor.processMetrics,
		processorhelper.WithCapabilities(consumerCapabilities))
}

// validateConfiguration validates the input configuration has all of the required fields for the processor
// An error is returned if there are any invalid inputs.
func validateConfiguration(config *Config) error {
	for _, transform := range config.Transforms {
		if transform.MetricIncludeFilter.Include == "" && transform.MetricName == "" {
			return fmt.Errorf("missing required field %q", IncludeFieldName)
		}

		if transform.MetricIncludeFilter.Include != "" && transform.MetricName != "" {
			return fmt.Errorf("cannot supply both %q and %q, use %q with %q match type", IncludeFieldName, MetricNameFieldName, IncludeFieldName, StrictMatchType)
		}

		if transform.MetricIncludeFilter.MatchType != "" && !transform.MetricIncludeFilter.MatchType.isValid() {
			return fmt.Errorf("%q must be in %q", MatchTypeFieldName, matchTypes)
		}

		if transform.MetricIncludeFilter.MatchType == RegexpMatchType {
			_, err := regexp.Compile(transform.MetricIncludeFilter.Include)
			if err != nil {
				return fmt.Errorf("%q, %w", IncludeFieldName, err)
			}
		}

		if !transform.Action.isValid() {
			return fmt.Errorf("%q must be in %q", ActionFieldName, actions)
		}

		if transform.Action == Insert && transform.NewName == "" {
			return fmt.Errorf("missing required field %q while %q is %v", NewNameFieldName, ActionFieldName, Insert)
		}

		if transform.Action == Group && transform.GroupResourceLabels == nil {
			return fmt.Errorf("missing required field %q while %q is %v", GroupResourceLabelsFieldName, ActionFieldName, Group)
		}

		if transform.AggregationType != "" && !transform.AggregationType.isValid() {
			return fmt.Errorf("%q must be in %q", AggregationTypeFieldName, aggregationTypes)
		}

		if transform.SubmatchCase != "" && !transform.SubmatchCase.isValid() {
			return fmt.Errorf("%q must be in %q", SubmatchCaseFieldName, submatchCases)
		}

		for i, op := range transform.Operations {
			if !op.Action.isValid() {
				return fmt.Errorf("operation %v: %q must be in %q", i+1, ActionFieldName, operationActions)
			}

			if op.Action == UpdateLabel && op.Label == "" {
				return fmt.Errorf("operation %v: missing required field %q while %q is %v", i+1, LabelFieldName, ActionFieldName, UpdateLabel)
			}
			if op.Action == AddLabel && op.NewLabel == "" {
				return fmt.Errorf("operation %v: missing required field %q while %q is %v", i+1, NewLabelFieldName, ActionFieldName, AddLabel)
			}
			if op.Action == AddLabel && op.NewValue == "" {
				return fmt.Errorf("operation %v: missing required field %q while %q is %v", i+1, NewValueFieldName, ActionFieldName, AddLabel)
			}
			if op.Action == ScaleValue && op.Scale == 0 {
				return fmt.Errorf("operation %v: missing required field %q while %q is %v", i+1, ScaleFieldName, ActionFieldName, ScaleValue)
			}

			if op.AggregationType != "" && !op.AggregationType.isValid() {
				return fmt.Errorf("operation %v: %q must be in %q", i+1, AggregationTypeFieldName, aggregationTypes)
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
			t.MetricIncludeFilter = FilterConfig{Include: t.MetricName}
			t.MetricName = ""
		}
		if t.MetricIncludeFilter.MatchType == "" {
			t.MetricIncludeFilter.MatchType = StrictMatchType
		}

		filter, err := createFilter(t.MetricIncludeFilter)
		if err != nil {
			return nil, err
		}

		helperT := internalTransform{
			MetricIncludeFilter: filter,
			Action:              t.Action,
			NewName:             t.NewName,
			GroupResourceLabels: t.GroupResourceLabels,
			AggregationType:     t.AggregationType,
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
	case StrictMatchType:
		matchers, err := getMatcherMap(filterConfig.MatchLabels, func(str string) (StringMatcher, error) { return strictMatcher(str), nil })
		if err != nil {
			return nil, err
		}
		return internalFilterStrict{include: filterConfig.Include, matchLabels: matchers}, nil
	case RegexpMatchType:
		matchers, err := getMatcherMap(filterConfig.MatchLabels, func(str string) (StringMatcher, error) { return regexp.Compile(str) })
		if err != nil {
			return nil, err
		}
		return internalFilterRegexp{include: regexp.MustCompile(filterConfig.Include), matchLabels: matchers}, nil
	}

	return nil, fmt.Errorf("invalid match type: %v", filterConfig.MatchType)
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

func getMatcherMap(strMap map[string]string, ctor func(string) (StringMatcher, error)) (map[string]StringMatcher, error) {
	out := make(map[string]StringMatcher)
	for k, v := range strMap {
		matcher, err := ctor(v)
		if err != nil {
			return nil, err
		}
		out[k] = matcher
	}
	return out, nil
}
