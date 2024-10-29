// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metricstransformprocessor

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/aggregateutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstransformprocessor/internal/metadata"
)

func TestType(t *testing.T) {
	factory := NewFactory()
	pType := factory.Type()
	assert.Equal(t, pType, metadata.Type)
}

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.Equal(t, &Config{}, cfg)
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestCreateProcessors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		configName   string
		succeed      bool
		errorMessage string
	}{
		{
			configName: "config_full.yaml",
			succeed:    true,
		},
		{
			configName:   "config_invalid_newname.yaml",
			succeed:      false,
			errorMessage: fmt.Sprintf("missing required field %q while %q is %v", newNameFieldName, actionFieldName, Insert),
		},
		{
			configName:   "config_invalid_group.yaml",
			succeed:      false,
			errorMessage: fmt.Sprintf("missing required field %q while %q is %v", groupResourceLabelsFieldName, actionFieldName, Group),
		},
		{
			configName:   "config_invalid_action.yaml",
			succeed:      false,
			errorMessage: fmt.Sprintf("%q must be in %q", actionFieldName, actions),
		},
		{
			configName:   "config_invalid_include.yaml",
			succeed:      false,
			errorMessage: fmt.Sprintf("missing required field %q", includeFieldName),
		},
		{
			configName:   "config_invalid_matchtype.yaml",
			succeed:      false,
			errorMessage: fmt.Sprintf("%q must be in %q", matchTypeFieldName, matchTypes),
		},
		{
			configName:   "config_invalid_label.yaml",
			succeed:      false,
			errorMessage: fmt.Sprintf("operation %v: missing required field %q while %q is %v", 1, labelFieldName, actionFieldName, updateLabel),
		},
		{
			configName:   "config_invalid_scale.yaml",
			succeed:      false,
			errorMessage: fmt.Sprintf("operation %v: missing required field %q while %q is %v", 1, scaleFieldName, actionFieldName, scaleValue),
		},
		{
			configName:   "config_invalid_regexp.yaml",
			succeed:      false,
			errorMessage: fmt.Sprintf("%q, error parsing regexp: missing closing ]: `[\\da`", includeFieldName),
		},
		{
			configName:   "config_invalid_aggregationtype.yaml",
			succeed:      false,
			errorMessage: fmt.Sprintf("%q must be in %q", aggregationTypeFieldName, aggregateutil.AggregationTypes),
		},
		{
			configName:   "config_invalid_operation_action.yaml",
			succeed:      false,
			errorMessage: fmt.Sprintf("operation %v: %q must be in %q", 1, actionFieldName, operationActions),
		},
		{
			configName:   "config_invalid_operation_aggregationtype.yaml",
			succeed:      false,
			errorMessage: fmt.Sprintf("operation %v: %q must be in %q", 1, aggregationTypeFieldName, aggregateutil.AggregationTypes),
		},
		{
			configName:   "config_invalid_submatchcase.yaml",
			succeed:      false,
			errorMessage: fmt.Sprintf("%q must be in %q", submatchCaseFieldName, submatchCases),
		},
	}

	for _, tt := range tests {
		cm, err := confmaptest.LoadConf(filepath.Join("testdata", tt.configName))
		require.NoError(t, err)

		for k := range cm.ToStringMap() {
			// Check if all processor variations that are defined in test config can be actually created
			t.Run(tt.configName, func(t *testing.T) {
				factory := NewFactory()
				cfg := factory.CreateDefaultConfig()

				sub, err := cm.Sub(k)
				require.NoError(t, err)
				require.NoError(t, sub.Unmarshal(cfg))

				tp, tErr := factory.CreateTraces(
					context.Background(),
					processortest.NewNopSettings(),
					cfg,
					consumertest.NewNop())
				// Not implemented error
				assert.Error(t, tErr)
				assert.Nil(t, tp)

				mp, mErr := factory.CreateMetrics(
					context.Background(),
					processortest.NewNopSettings(),
					cfg,
					consumertest.NewNop())
				if tt.succeed {
					assert.NotNil(t, mp)
					assert.NoError(t, mErr)
				} else {
					assert.EqualError(t, mErr, tt.errorMessage)
				}
			})
		}
	}
}

func TestFactory_validateConfiguration(t *testing.T) {
	v1 := Config{
		Transforms: []transform{
			{
				MetricIncludeFilter: FilterConfig{
					Include:   "mymetric",
					MatchType: strictMatchType,
				},
				Action: Update,
				Operations: []Operation{
					{
						Action:   addLabel,
						NewValue: "bar",
					},
				},
			},
		},
	}
	err := validateConfiguration(&v1)
	assert.Equal(t, "operation 1: missing required field \"new_label\" while \"action\" is add_label", err.Error())

	v2 := Config{
		Transforms: []transform{
			{
				MetricIncludeFilter: FilterConfig{
					Include:   "mymetric",
					MatchType: strictMatchType,
				},
				Action: Update,
				Operations: []Operation{
					{
						Action:   addLabel,
						NewLabel: "foo",
					},
				},
			},
		},
	}

	err = validateConfiguration(&v2)
	assert.Equal(t, "operation 1: missing required field \"new_value\" while \"action\" is add_label", err.Error())
}

func TestCreateProcessorsFilledData(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	oCfg := cfg.(*Config)

	oCfg.Transforms = []transform{
		{
			MetricIncludeFilter: FilterConfig{
				Include:   "name",
				MatchType: strictMatchType,
			},
			Action:  Update,
			NewName: "new-name",
			Operations: []Operation{
				{
					Action:   addLabel,
					NewLabel: "new-label",
					NewValue: "new-value {{version}}",
				},
				{
					Action:   updateLabel,
					Label:    "label",
					NewLabel: "new-label",
					ValueActions: []ValueAction{
						{
							Value:    "value",
							NewValue: "new/value {{version}}",
						},
					},
				},
				{
					Action:          aggregateLabels,
					LabelSet:        []string{"label1", "label2"},
					AggregationType: aggregateutil.Sum,
				},
				{
					Action:           aggregateLabelValues,
					Label:            "label",
					AggregatedValues: []string{"value1", "value2"},
					NewValue:         "new-value",
					AggregationType:  aggregateutil.Sum,
				},
			},
		},
	}

	expData := []internalTransform{
		{
			MetricIncludeFilter: internalFilterStrict{include: "name"},
			Action:              Update,
			NewName:             "new-name",
			Operations: []internalOperation{
				{
					configOperation: Operation{
						Action:   addLabel,
						NewLabel: "new-label",
						NewValue: "new-value v0.0.1",
					},
				},
				{
					configOperation: Operation{
						Action:   updateLabel,
						Label:    "label",
						NewLabel: "new-label",
						ValueActions: []ValueAction{
							{
								Value:    "value",
								NewValue: "new/value v0.0.1",
							},
						},
					},
					valueActionsMapping: map[string]string{"value": "new/value v0.0.1"},
				},
				{
					configOperation: Operation{
						Action:          aggregateLabels,
						LabelSet:        []string{"label1", "label2"},
						AggregationType: aggregateutil.Sum,
					},
					labelSetMap: map[string]bool{
						"label1": true,
						"label2": true,
					},
				},
				{
					configOperation: Operation{
						Action:           aggregateLabelValues,
						Label:            "label",
						AggregatedValues: []string{"value1", "value2"},
						NewValue:         "new-value",
						AggregationType:  aggregateutil.Sum,
					},
					aggregatedValuesSet: map[string]bool{
						"value1": true,
						"value2": true,
					},
				},
			},
		},
	}

	internalTransforms, err := buildHelperConfig(oCfg, "v0.0.1")
	assert.NoError(t, err)

	for i, expTr := range expData {
		mtpT := internalTransforms[i]
		assert.Equal(t, expTr.NewName, mtpT.NewName)
		assert.Equal(t, expTr.Action, mtpT.Action)
		assert.Equal(t, expTr.MetricIncludeFilter.(internalFilterStrict).include, mtpT.MetricIncludeFilter.(internalFilterStrict).include)
		for j, expOp := range expTr.Operations {
			mtpOp := mtpT.Operations[j]
			assert.Equal(t, expOp.configOperation, mtpOp.configOperation)
			assert.Equal(t, expOp.valueActionsMapping, mtpOp.valueActionsMapping)
			assert.Equal(t, expOp.labelSetMap, mtpOp.labelSetMap)
			assert.Equal(t, expOp.aggregatedValuesSet, mtpOp.aggregatedValuesSet)
		}
	}
}
