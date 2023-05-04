// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
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
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/processor/processortest"
)

func TestType(t *testing.T) {
	factory := NewFactory()
	pType := factory.Type()
	assert.Equal(t, pType, component.Type("metricstransform"))
}

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.Equal(t, cfg, &Config{})
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
			errorMessage: fmt.Sprintf("missing required field %q while %q is %v", NewNameFieldName, ActionFieldName, Insert),
		},
		{
			configName:   "config_invalid_group.yaml",
			succeed:      false,
			errorMessage: fmt.Sprintf("missing required field %q while %q is %v", GroupResourceLabelsFieldName, ActionFieldName, Group),
		},
		{
			configName:   "config_invalid_action.yaml",
			succeed:      false,
			errorMessage: fmt.Sprintf("%q must be in %q", ActionFieldName, actions),
		},
		{
			configName:   "config_invalid_include.yaml",
			succeed:      false,
			errorMessage: fmt.Sprintf("missing required field %q", IncludeFieldName),
		},
		{
			configName:   "config_invalid_matchtype.yaml",
			succeed:      false,
			errorMessage: fmt.Sprintf("%q must be in %q", MatchTypeFieldName, matchTypes),
		},
		{
			configName:   "config_invalid_label.yaml",
			succeed:      false,
			errorMessage: fmt.Sprintf("operation %v: missing required field %q while %q is %v", 1, LabelFieldName, ActionFieldName, UpdateLabel),
		},
		{
			configName:   "config_invalid_scale.yaml",
			succeed:      false,
			errorMessage: fmt.Sprintf("operation %v: missing required field %q while %q is %v", 1, ScaleFieldName, ActionFieldName, ScaleValue),
		},
		{
			configName:   "config_invalid_regexp.yaml",
			succeed:      false,
			errorMessage: fmt.Sprintf("%q, error parsing regexp: missing closing ]: `[\\da`", IncludeFieldName),
		},
		{
			configName:   "config_invalid_aggregationtype.yaml",
			succeed:      false,
			errorMessage: fmt.Sprintf("%q must be in %q", AggregationTypeFieldName, aggregationTypes),
		},
		{
			configName:   "config_invalid_operation_action.yaml",
			succeed:      false,
			errorMessage: fmt.Sprintf("operation %v: %q must be in %q", 1, ActionFieldName, operationActions),
		},
		{
			configName:   "config_invalid_operation_aggregationtype.yaml",
			succeed:      false,
			errorMessage: fmt.Sprintf("operation %v: %q must be in %q", 1, AggregationTypeFieldName, aggregationTypes),
		},
		{
			configName:   "config_invalid_submatchcase.yaml",
			succeed:      false,
			errorMessage: fmt.Sprintf("%q must be in %q", SubmatchCaseFieldName, submatchCases),
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
				require.NoError(t, component.UnmarshalConfig(sub, cfg))

				tp, tErr := factory.CreateTracesProcessor(
					context.Background(),
					processortest.NewNopCreateSettings(),
					cfg,
					consumertest.NewNop())
				// Not implemented error
				assert.Error(t, tErr)
				assert.Nil(t, tp)

				mp, mErr := factory.CreateMetricsProcessor(
					context.Background(),
					processortest.NewNopCreateSettings(),
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
		Transforms: []Transform{
			{
				MetricIncludeFilter: FilterConfig{
					Include:   "mymetric",
					MatchType: StrictMatchType,
				},
				Action: Update,
				Operations: []Operation{
					{
						Action:   AddLabel,
						NewValue: "bar",
					},
				},
			},
		},
	}
	err := validateConfiguration(&v1)
	assert.Equal(t, "operation 1: missing required field \"new_label\" while \"action\" is add_label", err.Error())

	v2 := Config{
		Transforms: []Transform{
			{
				MetricIncludeFilter: FilterConfig{
					Include:   "mymetric",
					MatchType: StrictMatchType,
				},
				Action: Update,
				Operations: []Operation{
					{
						Action:   AddLabel,
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

	oCfg.Transforms = []Transform{
		{
			MetricIncludeFilter: FilterConfig{
				Include:   "name",
				MatchType: StrictMatchType,
			},
			Action:  Update,
			NewName: "new-name",
			Operations: []Operation{
				{
					Action:   AddLabel,
					NewLabel: "new-label",
					NewValue: "new-value {{version}}",
				},
				{
					Action:   UpdateLabel,
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
					Action:          AggregateLabels,
					LabelSet:        []string{"label1", "label2"},
					AggregationType: Sum,
				},
				{
					Action:           AggregateLabelValues,
					Label:            "label",
					AggregatedValues: []string{"value1", "value2"},
					NewValue:         "new-value",
					AggregationType:  Sum,
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
						Action:   AddLabel,
						NewLabel: "new-label",
						NewValue: "new-value v0.0.1",
					},
				},
				{
					configOperation: Operation{
						Action:   UpdateLabel,
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
						Action:          AggregateLabels,
						LabelSet:        []string{"label1", "label2"},
						AggregationType: Sum,
					},
					labelSetMap: map[string]bool{
						"label1": true,
						"label2": true,
					},
				},
				{
					configOperation: Operation{
						Action:           AggregateLabelValues,
						Label:            "label",
						AggregatedValues: []string{"value1", "value2"},
						NewValue:         "new-value",
						AggregationType:  Sum,
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
