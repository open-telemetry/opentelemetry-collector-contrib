// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metricstransformprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstransformprocessor"

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processorhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/aggregateutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstransformprocessor/internal/metadata"
)

var consumerCapabilities = consumer.Capabilities{MutatesData: true}

// NewFactory returns a new factory for the Metrics transform processor.
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
	oCfg := cfg.(*Config)
	if err := validateConfiguration(oCfg); err != nil {
		return nil, err
	}

	hCfg, err := buildHelperConfig(oCfg, set.BuildInfo.Version)
	if err != nil {
		return nil, err
	}
	metricsProcessor := newMetricsTransformProcessor(set.Logger, hCfg)

	return processorhelper.NewMetrics(
		ctx,
		set,
		cfg,
		nextConsumer,
		metricsProcessor.processMetrics,
		processorhelper.WithCapabilities(consumerCapabilities))
}

// validateConfiguration validates the input configuration has all of the required fields for the processor
// An error is returned if there are any invalid inputs.
func validateConfiguration(config *Config) error {
	for _, transform := range config.Transforms {
		if transform.MetricIncludeFilter.Include == "" {
			return fmt.Errorf("missing required field %q", includeFieldName)
		}

		if transform.MetricIncludeFilter.MatchType != "" && !transform.MetricIncludeFilter.MatchType.isValid() {
			return fmt.Errorf("%q must be in %q", matchTypeFieldName, matchTypes)
		}

		if transform.MetricIncludeFilter.MatchType == regexpMatchType {
			_, err := regexp.Compile(transform.MetricIncludeFilter.Include)
			if err != nil {
				return fmt.Errorf("%q, %w", includeFieldName, err)
			}
		}

		if !transform.Action.isValid() {
			return fmt.Errorf("%q must be in %q", actionFieldName, actions)
		}

		if transform.Action == Insert && transform.NewName == "" {
			return fmt.Errorf("missing required field %q while %q is %v", newNameFieldName, actionFieldName, Insert)
		}

		if transform.Action == Group && transform.GroupResourceLabels == nil {
			return fmt.Errorf("missing required field %q while %q is %v", groupResourceLabelsFieldName, actionFieldName, Group)
		}

		if transform.AggregationType != "" && !transform.AggregationType.IsValid() {
			return fmt.Errorf("%q must be in %q", aggregationTypeFieldName, aggregateutil.AggregationTypes)
		}

		if transform.SubmatchCase != "" && !transform.SubmatchCase.isValid() {
			return fmt.Errorf("%q must be in %q", submatchCaseFieldName, submatchCases)
		}

		for i, op := range transform.Operations {
			if !op.Action.isValid() {
				return fmt.Errorf("operation %v: %q must be in %q", i+1, actionFieldName, operationActions)
			}

			if op.Action == updateLabel && op.Label == "" {
				return fmt.Errorf("operation %v: missing required field %q while %q is %v", i+1, labelFieldName, actionFieldName, updateLabel)
			}
			if op.Action == addLabel && op.NewLabel == "" {
				return fmt.Errorf("operation %v: missing required field %q while %q is %v", i+1, newLabelFieldName, actionFieldName, addLabel)
			}
			if op.Action == addLabel && op.NewValue == "" {
				return fmt.Errorf("operation %v: missing required field %q while %q is %v", i+1, newValueFieldName, actionFieldName, addLabel)
			}
			if op.Action == scaleValue && op.Scale == 0 {
				return fmt.Errorf("operation %v: missing required field %q while %q is %v", i+1, scaleFieldName, actionFieldName, scaleValue)
			}

			if op.AggregationType != "" && !op.AggregationType.IsValid() {
				return fmt.Errorf("operation %v: %q must be in %q", i+1, aggregationTypeFieldName, aggregateutil.AggregationTypes)
			}
		}
	}
	return nil
}

// buildHelperConfig constructs the maps that will be useful for the operations
func buildHelperConfig(config *Config, version string) ([]internalTransform, error) {
	helperDataTransforms := make([]internalTransform, len(config.Transforms))
	for i, t := range config.Transforms {
		if t.MetricIncludeFilter.MatchType == "" {
			t.MetricIncludeFilter.MatchType = strictMatchType
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
			switch op.Action {
			case aggregateLabels:
				mtpOp.labelSetMap = sliceToSet(op.LabelSet)
			case aggregateLabelValues:
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
	case strictMatchType:
		matchers, err := getMatcherMap(filterConfig.MatchLabels, func(str string) (StringMatcher, error) { return strictMatcher(str), nil })
		if err != nil {
			return nil, err
		}
		return internalFilterStrict{include: filterConfig.Include, attrMatchers: matchers}, nil
	case regexpMatchType:
		matchers, err := getMatcherMap(filterConfig.MatchLabels, func(str string) (StringMatcher, error) { return regexp.Compile(str) })
		if err != nil {
			return nil, err
		}
		return internalFilterRegexp{include: regexp.MustCompile(filterConfig.Include), attrMatchers: matchers}, nil
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
