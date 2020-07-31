// Copyright 2019, OpenTelemetry Authors
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

package translation

import (
	"sort"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	sfxpb "github.com/signalfx/com_signalfx_metrics_protobuf/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type byContent []*sfxpb.DataPoint

func (dps byContent) Len() int { return len(dps) }
func (dps byContent) Less(i, j int) bool {
	ib, _ := proto.Marshal(dps[i])
	jb, _ := proto.Marshal(dps[j])
	return string(ib) < string(jb)
}
func (dps byContent) Swap(i, j int) { dps[i], dps[j] = dps[j], dps[i] }

func TestNewMetricTranslator(t *testing.T) {
	tests := []struct {
		name              string
		trs               []Rule
		wantDimensionsMap map[string]string
		wantError         string
	}{
		{
			name: "invalid_rule",
			trs: []Rule{
				{
					Action: "invalid_rule",
				},
			},
			wantDimensionsMap: nil,
			wantError:         "unknown \"action\" value: \"invalid_rule\"",
		},
		{
			name: "rename_dimension_keys_valid",
			trs: []Rule{
				{
					Action: ActionRenameDimensionKeys,
					Mapping: map[string]string{
						"k8s.cluster.name": "kubernetes_cluster",
					},
				},
			},
			wantDimensionsMap: map[string]string{
				"k8s.cluster.name": "kubernetes_cluster",
			},
			wantError: "",
		},
		{
			name: "rename_dimension_keys_no_mapping",
			trs: []Rule{
				{
					Action: ActionRenameDimensionKeys,
				},
			},
			wantDimensionsMap: nil,
			wantError:         "field \"mapping\" is required for \"rename_dimension_keys\" translation rule",
		},
		{
			name: "rename_dimension_keys_many_actions_invalid",
			trs: []Rule{
				{
					Action: ActionRenameDimensionKeys,
					Mapping: map[string]string{
						"dimension1": "dimension2",
						"dimension3": "dimension4",
					},
				},
				{
					Action: ActionRenameDimensionKeys,
					Mapping: map[string]string{
						"dimension4": "dimension5",
					},
				},
			},
			wantDimensionsMap: nil,
			wantError:         "only one \"rename_dimension_keys\" translation rule can be specified",
		},
		{
			name: "rename_metric_valid",
			trs: []Rule{
				{
					Action: ActionRenameMetrics,
					Mapping: map[string]string{
						"metric1": "metric2",
					},
				},
			},
			wantDimensionsMap: nil,
			wantError:         "",
		},
		{
			name: "rename_metric_invalid",
			trs: []Rule{
				{
					Action: ActionRenameMetrics,
				},
			},
			wantDimensionsMap: nil,
			wantError:         "field \"mapping\" is required for \"rename_metrics\" translation rule",
		},
		{
			name: "rename_dimensions_and_metrics_valid",
			trs: []Rule{
				{
					Action: ActionRenameDimensionKeys,
					Mapping: map[string]string{
						"dimension1": "dimension2",
						"dimension3": "dimension4",
					},
				},
				{
					Action: ActionRenameMetrics,
					Mapping: map[string]string{
						"metric1": "metric2",
					},
				},
			},
			wantDimensionsMap: map[string]string{
				"dimension1": "dimension2",
				"dimension3": "dimension4",
			},
			wantError: "",
		},
		{
			name: "multiply_int_valid",
			trs: []Rule{
				{
					Action: ActionMultiplyInt,
					ScaleFactorsInt: map[string]int64{
						"metric1": 10,
					},
				},
			},
			wantDimensionsMap: nil,
			wantError:         "",
		},
		{
			name: "multiply_int_invalid",
			trs: []Rule{
				{
					Action: ActionMultiplyInt,
				},
			},
			wantDimensionsMap: nil,
			wantError:         "field \"scale_factors_int\" is required for \"multiply_int\" translation rule",
		},
		{
			name: "divide_int_valid",
			trs: []Rule{
				{
					Action: ActionDivideInt,
					ScaleFactorsInt: map[string]int64{
						"metric1": 10,
					},
				},
			},
			wantDimensionsMap: nil,
			wantError:         "",
		},
		{
			name: "divide_int_invalid_no_scale_factors",
			trs: []Rule{
				{
					Action: ActionDivideInt,
				},
			},
			wantDimensionsMap: nil,
			wantError:         "field \"scale_factors_int\" is required for \"divide_int\" translation rule",
		},
		{
			name: "divide_int_invalid_zero",
			trs: []Rule{
				{
					Action: ActionDivideInt,
					ScaleFactorsInt: map[string]int64{
						"metric1": 10,
						"metric2": 0,
					},
				},
			},
			wantDimensionsMap: nil,
			wantError:         "\"scale_factors_int\" for \"divide_int\" translation rule has 0 value for \"metric2\" metric",
		},
		{
			name: "multiply_float_valid",
			trs: []Rule{
				{
					Action: ActionMultiplyFloat,
					ScaleFactorsFloat: map[string]float64{
						"metric1": 0.1,
					},
				},
			},
			wantDimensionsMap: nil,
			wantError:         "",
		},
		{
			name: "multiply_float_invalid",
			trs: []Rule{
				{
					Action: ActionMultiplyFloat,
				},
			},
			wantDimensionsMap: nil,
			wantError:         "field \"scale_factors_float\" is required for \"multiply_float\" translation rule",
		},
		{
			name: "copy_metric_valid",
			trs: []Rule{
				{
					Action: ActionCopyMetrics,
					Mapping: map[string]string{
						"from_metric": "to_metric",
					},
				},
			},
			wantDimensionsMap: nil,
			wantError:         "",
		},
		{
			name: "copy_metric_invalid_no_mapping",
			trs: []Rule{
				{
					Action: ActionCopyMetrics,
				},
			},
			wantDimensionsMap: nil,
			wantError:         "field \"mapping\" is required for \"copy_metrics\" translation rule",
		},
		{
			name: "copy_metric_invalid_no_dimensions_filter",
			trs: []Rule{
				{
					Action: ActionCopyMetrics,
					Mapping: map[string]string{
						"metric1": "metric2",
					},
					DimensionKey: "dim1",
				},
			},
			wantDimensionsMap: nil,
			wantError: "\"dimension_values_filer\" has to be provided if \"dimension_key\" is set " +
				"for \"copy_metrics\" translation rule",
		},
		{
			name: "split_metric_valid",
			trs: []Rule{
				{
					Action:       ActionSplitMetric,
					MetricName:   "metric1",
					DimensionKey: "dim1",
					Mapping: map[string]string{
						"val1": "metric1.val1",
					},
				},
			},
			wantDimensionsMap: nil,
			wantError:         "",
		},
		{
			name: "split_metric_invalid",
			trs: []Rule{
				{
					Action:       ActionSplitMetric,
					MetricName:   "metric1",
					DimensionKey: "dim1",
				},
			},
			wantDimensionsMap: nil,
			wantError: "fields \"metric_name\", \"dimension_key\", and \"mapping\" are required " +
				"for \"split_metric\" translation rule",
		},
		{
			name: "convert_values_valid",
			trs: []Rule{
				{
					Action: ActionConvertValues,
					TypesMapping: map[string]MetricValueType{
						"val1": MetricValueTypeInt,
					},
				},
			},
			wantDimensionsMap: nil,
			wantError:         "",
		},
		{
			name: "convert_values_invalid_no_mapping",
			trs: []Rule{
				{
					Action: ActionConvertValues,
				},
			},
			wantDimensionsMap: nil,
			wantError:         "field \"types_mapping\" are required for \"convert_values\" translation rule",
		},
		{
			name: "convert_values_invalid_type",
			trs: []Rule{
				{
					Action: ActionConvertValues,
					TypesMapping: map[string]MetricValueType{
						"metric1": MetricValueType("invalid-type"),
					},
				},
			},
			wantDimensionsMap: nil,
			wantError:         "invalid value type \"invalid-type\" set for metric \"metric1\" in \"types_mapping\"",
		},
		{
			name: "aggregate_metric_valid",
			trs: []Rule{
				{
					Action:            ActionAggregateMetric,
					MetricName:        "metric",
					Dimensions:        []string{"dim"},
					AggregationMethod: AggregationMethodCount,
				},
			},
			wantDimensionsMap: nil,
			wantError:         "",
		},
		{
			name: "aggregate_metric_invalid_no_dimensions",
			trs: []Rule{
				{
					Action:            ActionAggregateMetric,
					MetricName:        "metric",
					AggregationMethod: AggregationMethodCount,
				},
			},
			wantDimensionsMap: nil,
			wantError: "fields \"metric_name\", \"dimensions\", and \"aggregation_method\" " +
				"are required for \"aggregate_metric\" translation rule",
		},
		{
			name: "aggregate_metric_invalid_aggregation_method",
			trs: []Rule{
				{
					Action:            ActionAggregateMetric,
					MetricName:        "metric",
					Dimensions:        []string{"dim"},
					AggregationMethod: AggregationMethod("invalid"),
				},
			},
			wantDimensionsMap: nil,
			wantError:         "invalid \"aggregation_method\": \"invalid\" provided for \"aggregate_metric\" translation rule",
		},
		{
			name: "divide_metrics_valid",
			trs: []Rule{
				{
					Action:     ActionDivideMetrics,
					MetricName: "metric",
					Mapping:    map[string]string{"metric0": "metric1"},
				},
			},
			wantDimensionsMap: nil,
			wantError:         "",
		},
		{
			name: "divide_metrics_invalid_missing_mapping",
			trs: []Rule{
				{
					Action:     ActionDivideMetrics,
					MetricName: "metric",
				},
			},
			wantDimensionsMap: nil,
			wantError:         "one mapping is required for \"divide_metrics\", found 0",
		},
		{
			name: "divide_metrics_invalid_missing_metric_name",
			trs: []Rule{
				{
					Action:  ActionDivideMetrics,
					Mapping: map[string]string{"metric0": "metric1"},
				},
			},
			wantDimensionsMap: nil,
			wantError:         "field \"metric_name\" is required for \"divide_metrics\" translation rule",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mt, err := NewMetricTranslator(tt.trs)
			if tt.wantError == "" {
				require.NoError(t, err)
				require.NotNil(t, mt)
				assert.Equal(t, tt.trs, mt.rules)
				assert.Equal(t, tt.wantDimensionsMap, mt.dimensionsMap)
			} else {
				require.Error(t, err)
				assert.Equal(t, err.Error(), tt.wantError)
				require.Nil(t, mt)
			}
		})
	}
}

func TestTranslateDataPoints(t *testing.T) {
	msec := time.Now().Unix() * 1e3
	gaugeType := sfxpb.MetricType_GAUGE

	tests := []struct {
		name string
		trs  []Rule
		dps  []*sfxpb.DataPoint
		want []*sfxpb.DataPoint
	}{
		{
			name: "rename_dimension_keys",
			trs: []Rule{
				{
					Action: ActionRenameDimensionKeys,
					Mapping: map[string]string{
						"old_dimension": "new_dimension",
						"old.dimension": "new.dimension",
					},
				},
			},
			dps: []*sfxpb.DataPoint{
				{
					Metric:    "single",
					Timestamp: msec,
					Value: sfxpb.Datum{
						IntValue: generateIntPtr(13),
					},
					MetricType: &gaugeType,
					Dimensions: []*sfxpb.Dimension{
						{
							Key:   "old_dimension",
							Value: "value1",
						},
						{
							Key:   "old.dimension",
							Value: "value2",
						},
						{
							Key:   "dimension",
							Value: "value3",
						},
					},
				},
			},
			want: []*sfxpb.DataPoint{
				{
					Metric:    "single",
					Timestamp: msec,
					Value: sfxpb.Datum{
						IntValue: generateIntPtr(13),
					},
					MetricType: &gaugeType,
					Dimensions: []*sfxpb.Dimension{
						{
							Key:   "new_dimension",
							Value: "value1",
						},
						{
							Key:   "new.dimension",
							Value: "value2",
						},
						{
							Key:   "dimension",
							Value: "value3",
						},
					},
				},
			},
		},
		{
			name: "rename_metric",
			trs: []Rule{
				{
					Action: ActionRenameMetrics,
					Mapping: map[string]string{
						"k8s/container/mem/usage": "container_memory_usage_bytes",
					},
				},
			},
			dps: []*sfxpb.DataPoint{
				{
					Metric:    "k8s/container/mem/usage",
					Timestamp: msec,
					Value: sfxpb.Datum{
						IntValue: generateIntPtr(13),
					},
					MetricType: &gaugeType,
					Dimensions: []*sfxpb.Dimension{},
				},
			},
			want: []*sfxpb.DataPoint{
				{
					Metric:    "container_memory_usage_bytes",
					Timestamp: msec,
					Value: sfxpb.Datum{
						IntValue: generateIntPtr(13),
					},
					MetricType: &gaugeType,
					Dimensions: []*sfxpb.Dimension{},
				},
			},
		},
		{
			name: "multiply_int",
			trs: []Rule{
				{
					Action: ActionMultiplyInt,
					ScaleFactorsInt: map[string]int64{
						"metric1": 100,
					},
				},
			},
			dps: []*sfxpb.DataPoint{
				{
					Metric:    "metric1",
					Timestamp: msec,
					Value: sfxpb.Datum{
						IntValue: generateIntPtr(13),
					},
					MetricType: &gaugeType,
					Dimensions: []*sfxpb.Dimension{},
				},
			},
			want: []*sfxpb.DataPoint{
				{
					Metric:    "metric1",
					Timestamp: msec,
					Value: sfxpb.Datum{
						IntValue: generateIntPtr(1300),
					},
					MetricType: &gaugeType,
					Dimensions: []*sfxpb.Dimension{},
				},
			},
		},
		{
			name: "divide_int",
			trs: []Rule{
				{
					Action: ActionDivideInt,
					ScaleFactorsInt: map[string]int64{
						"metric1": 100,
					},
				},
			},
			dps: []*sfxpb.DataPoint{
				{
					Metric:    "metric1",
					Timestamp: msec,
					Value: sfxpb.Datum{
						IntValue: generateIntPtr(1300),
					},
					MetricType: &gaugeType,
					Dimensions: []*sfxpb.Dimension{},
				},
			},
			want: []*sfxpb.DataPoint{
				{
					Metric:    "metric1",
					Timestamp: msec,
					Value: sfxpb.Datum{
						IntValue: generateIntPtr(13),
					},
					MetricType: &gaugeType,
					Dimensions: []*sfxpb.Dimension{},
				},
			},
		},
		{
			name: "multiply_float",
			trs: []Rule{
				{
					Action: ActionMultiplyFloat,
					ScaleFactorsFloat: map[string]float64{
						"metric1": 0.1,
					},
				},
			},
			dps: []*sfxpb.DataPoint{
				{
					Metric:    "metric1",
					Timestamp: msec,
					Value: sfxpb.Datum{
						DoubleValue: generateFloatPtr(0.9),
					},
					MetricType: &gaugeType,
					Dimensions: []*sfxpb.Dimension{},
				},
			},
			want: []*sfxpb.DataPoint{
				{
					Metric:    "metric1",
					Timestamp: msec,
					Value: sfxpb.Datum{
						DoubleValue: generateFloatPtr(0.09),
					},
					MetricType: &gaugeType,
					Dimensions: []*sfxpb.Dimension{},
				},
			},
		},
		{
			name: "copy_metric",
			trs: []Rule{
				{
					Action: ActionCopyMetrics,
					Mapping: map[string]string{
						"metric1": "metric2",
					},
				},
			},
			dps: []*sfxpb.DataPoint{
				{
					Metric:    "metric1",
					Timestamp: msec,
					Value: sfxpb.Datum{
						IntValue: generateIntPtr(9),
					},
					MetricType: &gaugeType,
					Dimensions: []*sfxpb.Dimension{},
				},
			},
			want: []*sfxpb.DataPoint{
				{
					Metric:    "metric1",
					Timestamp: msec,
					Value: sfxpb.Datum{
						IntValue: generateIntPtr(9),
					},
					MetricType: &gaugeType,
					Dimensions: []*sfxpb.Dimension{},
				},
				{
					Metric:    "metric2",
					Timestamp: msec,
					Value: sfxpb.Datum{
						IntValue: generateIntPtr(9),
					},
					MetricType: &gaugeType,
					Dimensions: []*sfxpb.Dimension{},
				},
			},
		},
		{
			name: "copy_with_dimension_filter",
			trs: []Rule{
				{
					Action: ActionCopyMetrics,
					Mapping: map[string]string{
						"metric1": "metric2",
					},
					DimensionKey: "dim1",
					DimensionValues: map[string]bool{
						"val1": true,
						"val2": true,
					},
				},
			},
			dps: []*sfxpb.DataPoint{
				{
					Metric:    "metric1",
					Timestamp: msec,
					Value: sfxpb.Datum{
						IntValue: generateIntPtr(9),
					},
					Dimensions: []*sfxpb.Dimension{
						{
							Key:   "dim1",
							Value: "val1",
						},
					},
				},
				{
					Metric:    "metric1",
					Timestamp: msec,
					Value: sfxpb.Datum{
						IntValue: generateIntPtr(9),
					},
					Dimensions: []*sfxpb.Dimension{
						{
							Key:   "dim1",
							Value: "val2",
						},
					},
				},
				// must not be copied
				{
					Metric:    "metric1",
					Timestamp: msec,
					Value: sfxpb.Datum{
						IntValue: generateIntPtr(9),
					},
					MetricType: &gaugeType,
					Dimensions: []*sfxpb.Dimension{
						{
							Key:   "dim1",
							Value: "val3",
						},
					},
				},
			},
			want: []*sfxpb.DataPoint{
				{
					Metric:    "metric1",
					Timestamp: msec,
					Value: sfxpb.Datum{
						IntValue: generateIntPtr(9),
					},
					Dimensions: []*sfxpb.Dimension{
						{
							Key:   "dim1",
							Value: "val1",
						},
					},
				},
				{
					Metric:    "metric1",
					Timestamp: msec,
					Value: sfxpb.Datum{
						IntValue: generateIntPtr(9),
					},
					Dimensions: []*sfxpb.Dimension{
						{
							Key:   "dim1",
							Value: "val2",
						},
					},
				},
				{
					Metric:    "metric1",
					Timestamp: msec,
					Value: sfxpb.Datum{
						IntValue: generateIntPtr(9),
					},
					MetricType: &gaugeType,
					Dimensions: []*sfxpb.Dimension{
						{
							Key:   "dim1",
							Value: "val3",
						},
					},
				},
				{
					Metric:    "metric2",
					Timestamp: msec,
					Value: sfxpb.Datum{
						IntValue: generateIntPtr(9),
					},
					Dimensions: []*sfxpb.Dimension{
						{
							Key:   "dim1",
							Value: "val1",
						},
					},
				},
				{
					Metric:    "metric2",
					Timestamp: msec,
					Value: sfxpb.Datum{
						IntValue: generateIntPtr(9),
					},
					Dimensions: []*sfxpb.Dimension{
						{
							Key:   "dim1",
							Value: "val2",
						},
					},
				},
			},
		},
		{
			name: "copy_and_rename",
			trs: []Rule{
				{
					Action: ActionCopyMetrics,
					Mapping: map[string]string{
						"metric1": "metric2",
					},
				},
				{
					Action: ActionRenameMetrics,
					Mapping: map[string]string{
						"metric2": "metric3",
					},
				},
			},
			dps: []*sfxpb.DataPoint{
				{
					Metric:    "metric1",
					Timestamp: msec,
					Value: sfxpb.Datum{
						IntValue: generateIntPtr(9),
					},
				},
			},
			want: []*sfxpb.DataPoint{
				{
					Metric:    "metric1",
					Timestamp: msec,
					Value: sfxpb.Datum{
						IntValue: generateIntPtr(9),
					},
				},
				{
					Metric:    "metric3",
					Timestamp: msec,
					Value: sfxpb.Datum{
						IntValue: generateIntPtr(9),
					},
				},
			},
		},
		{
			name: "split_metric",
			trs: []Rule{
				{
					Action:       ActionSplitMetric,
					MetricName:   "metric1",
					DimensionKey: "dim1",
					Mapping: map[string]string{
						"val1": "metric1.dim1-val1",
						"val2": "metric1.dim1-val2",
					},
				},
			},
			dps: []*sfxpb.DataPoint{
				{
					Metric:    "metric1",
					Timestamp: msec,
					Value: sfxpb.Datum{
						IntValue: generateIntPtr(9),
					},
					MetricType: &gaugeType,
					Dimensions: []*sfxpb.Dimension{
						{
							Key:   "dim1",
							Value: "val1",
						},
						{
							Key:   "dim2",
							Value: "val2",
						},
					},
				},
				{
					Metric:    "metric1",
					Timestamp: msec,
					Value: sfxpb.Datum{
						IntValue: generateIntPtr(9),
					},
					MetricType: &gaugeType,
					Dimensions: []*sfxpb.Dimension{
						{
							Key:   "dim1",
							Value: "val2",
						},
						{
							Key:   "dim2",
							Value: "val2-aleternate",
						},
					},
				},
				// datapoint with no dimensions, should not be changed
				{
					Metric:    "metric1",
					Timestamp: msec,
					Value: sfxpb.Datum{
						IntValue: generateIntPtr(9),
					},
					MetricType: &gaugeType,
					Dimensions: []*sfxpb.Dimension{},
				},
				// datapoint with another dimension key, should not be changed
				{
					Metric:    "metric1",
					Timestamp: msec,
					Value: sfxpb.Datum{
						IntValue: generateIntPtr(9),
					},
					MetricType: &gaugeType,
					Dimensions: []*sfxpb.Dimension{
						{
							Key:   "dim2",
							Value: "val2",
						},
					},
				},
				// datapoint with dimension value not matching the mapping, should not be changed
				{
					Metric:    "metric1",
					Timestamp: msec,
					Value: sfxpb.Datum{
						IntValue: generateIntPtr(9),
					},
					MetricType: &gaugeType,
					Dimensions: []*sfxpb.Dimension{
						{
							Key:   "dim1",
							Value: "val3",
						},
					},
				},
			},
			want: []*sfxpb.DataPoint{
				{
					Metric:    "metric1.dim1-val1",
					Timestamp: msec,
					Value: sfxpb.Datum{
						IntValue: generateIntPtr(9),
					},
					MetricType: &gaugeType,
					Dimensions: []*sfxpb.Dimension{
						{
							Key:   "dim2",
							Value: "val2",
						},
					},
				},
				{
					Metric:    "metric1.dim1-val2",
					Timestamp: msec,
					Value: sfxpb.Datum{
						IntValue: generateIntPtr(9),
					},
					MetricType: &gaugeType,
					Dimensions: []*sfxpb.Dimension{
						{
							Key:   "dim2",
							Value: "val2-aleternate",
						},
					},
				},
				{
					Metric:    "metric1",
					Timestamp: msec,
					Value: sfxpb.Datum{
						IntValue: generateIntPtr(9),
					},
					MetricType: &gaugeType,
					Dimensions: []*sfxpb.Dimension{},
				},
				{
					Metric:    "metric1",
					Timestamp: msec,
					Value: sfxpb.Datum{
						IntValue: generateIntPtr(9),
					},
					MetricType: &gaugeType,
					Dimensions: []*sfxpb.Dimension{
						{
							Key:   "dim2",
							Value: "val2",
						},
					},
				},
				{
					Metric:    "metric1",
					Timestamp: msec,
					Value: sfxpb.Datum{
						IntValue: generateIntPtr(9),
					},
					MetricType: &gaugeType,
					Dimensions: []*sfxpb.Dimension{
						{
							Key:   "dim1",
							Value: "val3",
						},
					},
				},
			},
		},
		{
			name: "convert_values",
			trs: []Rule{
				{
					Action: ActionConvertValues,
					TypesMapping: map[string]MetricValueType{
						"metric1": MetricValueTypeInt,
						"metric2": MetricValueTypeDouble,
						"metric3": MetricValueTypeInt,
					},
				},
			},
			dps: []*sfxpb.DataPoint{
				{
					Metric:    "metric1",
					Timestamp: msec,
					Value: sfxpb.Datum{
						DoubleValue: generateFloatPtr(9.1),
					},
					MetricType: &gaugeType,
				},
				{
					Metric:    "metric2",
					Timestamp: msec,
					Value: sfxpb.Datum{
						IntValue: generateIntPtr(0),
					},
					MetricType: &gaugeType,
				},
				{
					Metric:    "metric3",
					Timestamp: msec,
					Value: sfxpb.Datum{
						IntValue: generateIntPtr(12),
					},
					MetricType: &gaugeType,
					Dimensions: []*sfxpb.Dimension{},
				},
			},
			want: []*sfxpb.DataPoint{
				{
					Metric:    "metric1",
					Timestamp: msec,
					Value: sfxpb.Datum{
						IntValue: generateIntPtr(9),
					},
					MetricType: &gaugeType,
				},
				{
					Metric:    "metric2",
					Timestamp: msec,
					Value: sfxpb.Datum{
						DoubleValue: generateFloatPtr(0),
					},
					MetricType: &gaugeType,
				},
				{
					Metric:    "metric3",
					Timestamp: msec,
					Value: sfxpb.Datum{
						IntValue: generateIntPtr(12),
					},
					MetricType: &gaugeType,
					Dimensions: []*sfxpb.Dimension{},
				},
			},
		},
		{
			name: "aggregate_metric_count",
			trs: []Rule{
				{
					Action:            ActionAggregateMetric,
					MetricName:        "metric1",
					Dimensions:        []string{"dim1", "dim2"},
					AggregationMethod: "count",
				},
			},
			dps: []*sfxpb.DataPoint{
				{
					Metric:    "metric1",
					Timestamp: msec,
					Value: sfxpb.Datum{
						IntValue: generateIntPtr(9),
					},
					Dimensions: []*sfxpb.Dimension{
						{
							Key:   "dim1",
							Value: "val1",
						},
						{
							Key:   "dim2",
							Value: "val1",
						},
						{
							Key:   "dim3",
							Value: "different",
						},
						{
							Key:   "dim4",
							Value: "just-one",
						},
					},
				},
				{
					Metric:    "metric1",
					Timestamp: msec,
					Value: sfxpb.Datum{
						IntValue: generateIntPtr(8),
					},
					Dimensions: []*sfxpb.Dimension{
						{
							Key:   "dim1",
							Value: "val1",
						},
						{
							Key:   "dim2",
							Value: "val1",
						},
						{
							Key:   "dim3",
							Value: "another",
						},
					},
				},
				{
					Metric:    "metric1",
					Timestamp: msec,
					Value: sfxpb.Datum{
						IntValue: generateIntPtr(2),
					},
					Dimensions: []*sfxpb.Dimension{
						{
							Key:   "dim1",
							Value: "val2",
						},
						{
							Key:   "dim2",
							Value: "val2",
						},
					},
				},
				{
					Metric:    "another-metric",
					Timestamp: msec,
					Value: sfxpb.Datum{
						IntValue: generateIntPtr(23),
					},
					Dimensions: []*sfxpb.Dimension{
						{
							Key:   "dim1",
							Value: "val1",
						},
					},
				},

				// invalid datapoint without required dimension, must be dropped
				{
					Metric:    "metric1",
					Timestamp: msec,
					Value: sfxpb.Datum{
						IntValue: generateIntPtr(2),
					},
					Dimensions: []*sfxpb.Dimension{
						{
							Key:   "dim2",
							Value: "val2",
						},
					},
				},
			},
			want: []*sfxpb.DataPoint{
				{
					Metric:    "another-metric",
					Timestamp: msec,
					Value: sfxpb.Datum{
						IntValue: generateIntPtr(23),
					},
					Dimensions: []*sfxpb.Dimension{
						{
							Key:   "dim1",
							Value: "val1",
						},
					},
				},
				{
					Metric:     "metric1",
					Timestamp:  msec,
					MetricType: &gaugeType,
					Value: sfxpb.Datum{
						IntValue: generateIntPtr(2),
					},
					Dimensions: []*sfxpb.Dimension{
						{
							Key:   "dim1",
							Value: "val1",
						},
						{
							Key:   "dim2",
							Value: "val1",
						},
					},
				},
				{
					Metric:     "metric1",
					Timestamp:  msec,
					MetricType: &gaugeType,
					Value: sfxpb.Datum{
						IntValue: generateIntPtr(1),
					},
					Dimensions: []*sfxpb.Dimension{
						{
							Key:   "dim1",
							Value: "val2",
						},
						{
							Key:   "dim2",
							Value: "val2",
						},
					},
				},
			},
		},
		{
			name: "aggregate_metric_sum_int",
			trs: []Rule{
				{
					Action:            ActionAggregateMetric,
					MetricName:        "metric1",
					Dimensions:        []string{"dim1"},
					AggregationMethod: "sum",
				},
			},
			dps: []*sfxpb.DataPoint{
				{
					Metric:     "metric1",
					Timestamp:  msec,
					MetricType: &gaugeType,
					Value: sfxpb.Datum{
						IntValue: generateIntPtr(9),
					},
					Dimensions: []*sfxpb.Dimension{
						{
							Key:   "dim1",
							Value: "val1",
						},
						{
							Key:   "dim2",
							Value: "val1",
						},
					},
				},
				{
					Metric:     "metric1",
					Timestamp:  msec,
					MetricType: &gaugeType,
					Value: sfxpb.Datum{
						IntValue: generateIntPtr(8),
					},
					Dimensions: []*sfxpb.Dimension{
						{
							Key:   "dim1",
							Value: "val1",
						},
						{
							Key:   "dim2",
							Value: "val1",
						},
					},
				},
				{
					Metric:     "metric1",
					Timestamp:  msec,
					MetricType: &gaugeType,
					Value: sfxpb.Datum{
						IntValue: generateIntPtr(2),
					},
					Dimensions: []*sfxpb.Dimension{
						{
							Key:   "dim1",
							Value: "val2",
						},
						{
							Key:   "dim2",
							Value: "val2",
						},
					},
				},
			},
			want: []*sfxpb.DataPoint{
				{
					Metric:     "metric1",
					Timestamp:  msec,
					MetricType: &gaugeType,
					Value: sfxpb.Datum{
						IntValue: generateIntPtr(17),
					},
					Dimensions: []*sfxpb.Dimension{
						{
							Key:   "dim1",
							Value: "val1",
						},
					},
				},
				{
					Metric:     "metric1",
					Timestamp:  msec,
					MetricType: &gaugeType,
					Value: sfxpb.Datum{
						IntValue: generateIntPtr(2),
					},
					Dimensions: []*sfxpb.Dimension{
						{
							Key:   "dim1",
							Value: "val2",
						},
					},
				},
			},
		},
		{
			name: "aggregate_metric_sum_float",
			trs: []Rule{
				{
					Action:            ActionAggregateMetric,
					MetricName:        "metric1",
					AggregationMethod: "sum",
					Dimensions:        []string{"dim1"},
				},
			},
			dps: []*sfxpb.DataPoint{
				{
					Metric:     "metric1",
					Timestamp:  msec,
					MetricType: &gaugeType,
					Value: sfxpb.Datum{
						DoubleValue: generateFloatPtr(1.2),
					},
					Dimensions: []*sfxpb.Dimension{
						{
							Key:   "dim1",
							Value: "val1",
						},
					},
				},
				{
					Metric:     "metric1",
					Timestamp:  msec,
					MetricType: &gaugeType,
					Value: sfxpb.Datum{
						DoubleValue: generateFloatPtr(2.2),
					},
					Dimensions: []*sfxpb.Dimension{
						{
							Key:   "dim1",
							Value: "val1",
						},
					},
				},
			},
			want: []*sfxpb.DataPoint{
				{
					Metric:     "metric1",
					Timestamp:  msec,
					MetricType: &gaugeType,
					Value: sfxpb.Datum{
						DoubleValue: generateFloatPtr(3.4),
					},
					Dimensions: []*sfxpb.Dimension{
						{
							Key:   "dim1",
							Value: "val1",
						},
					},
				},
			},
		}, {
			name: "divide_metrics",
			trs: []Rule{{
				Action:     ActionDivideMetrics,
				MetricName: "new_metric",
				Mapping:    map[string]string{"metric0": "metric1"},
			}},
			dps: []*sfxpb.DataPoint{
				{
					Metric:     "metric0",
					Timestamp:  msec,
					MetricType: &gaugeType,
					Value: sfxpb.Datum{
						IntValue: generateIntPtr(42),
					},
					Dimensions: []*sfxpb.Dimension{{
						Key:   "dim1",
						Value: "val1",
					}},
				},
				{
					Metric:     "metric1",
					Timestamp:  msec,
					MetricType: &gaugeType,
					Value: sfxpb.Datum{
						IntValue: generateIntPtr(84),
					},
					Dimensions: []*sfxpb.Dimension{{
						Key:   "dim2",
						Value: "val2",
					}},
				},
			},
			want: []*sfxpb.DataPoint{
				{
					Metric:     "metric0",
					Timestamp:  msec,
					MetricType: &gaugeType,
					Value: sfxpb.Datum{
						IntValue: generateIntPtr(42),
					},
					Dimensions: []*sfxpb.Dimension{{
						Key:   "dim1",
						Value: "val1",
					}},
				},
				{
					Metric:     "metric1",
					Timestamp:  msec,
					MetricType: &gaugeType,
					Value: sfxpb.Datum{
						IntValue: generateIntPtr(84),
					},
					Dimensions: []*sfxpb.Dimension{{
						Key:   "dim2",
						Value: "val2",
					}},
				},
				{
					Metric:     "new_metric",
					Timestamp:  msec,
					MetricType: &gaugeType,
					Value: sfxpb.Datum{
						DoubleValue: generateFloatPtr(0.5),
					},
					Dimensions: []*sfxpb.Dimension{{
						Key:   "dim1",
						Value: "val1",
					}},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mt, err := NewMetricTranslator(tt.trs)
			require.NoError(t, err)
			assert.NotEqualValues(t, tt.want, tt.dps)
			got := mt.TranslateDataPoints(zap.NewNop(), tt.dps)

			// Handle float values separately
			for i, dp := range got {
				if dp.GetValue().DoubleValue != nil {
					assert.InDelta(t, *tt.want[i].GetValue().DoubleValue, *dp.GetValue().DoubleValue, 0.00000001)
					*dp.GetValue().DoubleValue = *tt.want[i].GetValue().DoubleValue
				}
			}

			// Sort metrics to handle not deterministic order from aggregation
			if tt.trs[0].Action == ActionAggregateMetric {
				sort.Sort(byContent(tt.want))
				sort.Sort(byContent(got))
			}

			assert.EqualValues(t, tt.want, got)
		})
	}
}

func TestTestTranslateDimension(t *testing.T) {
	mt, err := NewMetricTranslator([]Rule{
		{
			Action: ActionRenameDimensionKeys,
			Mapping: map[string]string{
				"old_dimension": "new_dimension",
				"old.dimension": "new.dimension",
			},
		},
	})
	require.NoError(t, err)

	assert.Equal(t, "new_dimension", mt.TranslateDimension("old_dimension"))
	assert.Equal(t, "new.dimension", mt.TranslateDimension("old.dimension"))
	assert.Equal(t, "another_dimension", mt.TranslateDimension("another_dimension"))

	// Test no rename_dimension_keys translation rule
	mt, err = NewMetricTranslator([]Rule{})
	require.NoError(t, err)
	assert.Equal(t, "old_dimension", mt.TranslateDimension("old_dimension"))
}

func generateIntPtr(i int) *int64 {
	var iPtr = int64(i)
	return &iPtr
}

func generateFloatPtr(i float64) *float64 {
	var iPtr = i
	return &iPtr
}
