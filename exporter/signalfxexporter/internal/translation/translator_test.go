// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translation

import (
	"encoding/json"
	"sort"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	sfxpb "github.com/signalfx/com_signalfx_metrics_protobuf/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

type byContent []*sfxpb.DataPoint

func (dps byContent) Len() int { return len(dps) }
func (dps byContent) Less(i, j int) bool {
	ib, _ := proto.Marshal(dps[i])
	jb, _ := proto.Marshal(dps[j])
	return string(ib) < string(jb)
}
func (dps byContent) Swap(i, j int) { dps[i], dps[j] = dps[j], dps[i] }

type byDimensions []*sfxpb.Dimension

func (dims byDimensions) Len() int { return len(dims) }
func (dims byDimensions) Less(i, j int) bool {
	ib, err := json.Marshal(dims[i])
	if err != nil {
		panic(err)
	}
	jb, err := json.Marshal(dims[j])
	if err != nil {
		panic(err)
	}
	return string(ib) < string(jb)
}
func (dims byDimensions) Swap(i, j int) { dims[i], dims[j] = dims[j], dims[i] }

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
			wantError:         "only one \"rename_dimension_keys\" translation rule without \"metric_names\" can be specified",
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
			name: "rename_metric_valid_add_dimensions",
			trs: []Rule{
				{
					Action: ActionRenameMetrics,
					Mapping: map[string]string{
						"metric1": "metric2",
					},
					AddDimensions: map[string]string{
						"dim1": "val1",
					},
				},
			},
			wantDimensionsMap: nil,
			wantError:         "",
		},
		{
			name: "rename_metric_valid_copy_dimensions",
			trs: []Rule{
				{
					Action: ActionRenameMetrics,
					Mapping: map[string]string{
						"metric1": "metric2",
					},
					CopyDimensions: map[string]string{
						"dim1": "dim2",
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
			name: "rename_metric_invalid_copy_dimensions",
			trs: []Rule{
				{
					Action: ActionRenameMetrics,
					Mapping: map[string]string{
						"metric1": "metric2",
					},
					CopyDimensions: map[string]string{
						"dim1": "",
					},
				},
			},
			wantDimensionsMap: nil,
			wantError:         `mapping "copy_dimensions" for "rename_metrics" translation rule must not contain empty string keys or values`,
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
					WithoutDimensions: []string{"dim"},
					AggregationMethod: AggregationMethodCount,
				},
			},
			wantDimensionsMap: nil,
			wantError:         "",
		},
		{
			name: "aggregate_metric_invalid_no_dimension_key",
			trs: []Rule{
				{
					Action:            ActionAggregateMetric,
					MetricName:        "metric",
					AggregationMethod: AggregationMethodCount,
				},
			},
			wantDimensionsMap: nil,
			wantError: "fields \"metric_name\", \"without_dimensions\", and \"aggregation_method\" " +
				"are required for \"aggregate_metric\" translation rule",
		},
		{
			name: "aggregate_metric_invalid_aggregation_method",
			trs: []Rule{
				{
					Action:            ActionAggregateMetric,
					MetricName:        "metric",
					WithoutDimensions: []string{"dim"},
					AggregationMethod: AggregationMethod("invalid"),
				},
			},
			wantDimensionsMap: nil,
			wantError:         "invalid \"aggregation_method\": \"invalid\" provided for \"aggregate_metric\" translation rule",
		},
		{
			name: "calculate_new_metric_valid",
			trs: []Rule{
				{
					Action:         ActionCalculateNewMetric,
					MetricName:     "metric",
					Operand1Metric: "op1_metric",
					Operand2Metric: "op2_metric",
					Operator:       MetricOperatorDivision,
				},
			},
			wantDimensionsMap: nil,
			wantError:         "",
		},
		{
			name: "divide_metrics_invalid_missing_metric_name",
			trs: []Rule{
				{
					Action:         ActionCalculateNewMetric,
					Operand1Metric: "op1_metric",
					Operand2Metric: "op2_metric",
					Operator:       MetricOperatorDivision,
				},
			},
			wantDimensionsMap: nil,
			wantError: `fields "metric_name", "operand1_metric", "operand2_metric", and "operator" ` +
				`are required for "calculate_new_metric" translation rule`,
		},
		{
			name: "divide_metrics_invalid_missing_op_1",
			trs: []Rule{
				{
					Action:         ActionCalculateNewMetric,
					MetricName:     "metric",
					Operand2Metric: "op2_metric",
					Operator:       MetricOperatorDivision,
				},
			},
			wantDimensionsMap: nil,
			wantError: `fields "metric_name", "operand1_metric", "operand2_metric", and "operator" ` +
				`are required for "calculate_new_metric" translation rule`,
		},
		{
			name: "divide_metrics_invalid_missing_op_2",
			trs: []Rule{
				{
					Action:         ActionCalculateNewMetric,
					MetricName:     "metric",
					Operand1Metric: "op1_metric",
					Operator:       MetricOperatorDivision,
				},
			},
			wantDimensionsMap: nil,
			wantError: `fields "metric_name", "operand1_metric", "operand2_metric", and "operator" ` +
				`are required for "calculate_new_metric" translation rule`,
		},
		{
			name: "calculate_new_metric_missing_operator",
			trs: []Rule{
				{
					Action:         ActionCalculateNewMetric,
					MetricName:     "metric",
					Operand1Metric: "op1_metric",
					Operand2Metric: "op2_metric",
				},
			},
			wantDimensionsMap: nil,
			wantError: `fields "metric_name", "operand1_metric", "operand2_metric", and "operator" ` +
				`are required for "calculate_new_metric" translation rule`,
		},
		{
			name: "drop_metrics_valid",
			trs: []Rule{
				{
					Action:      ActionDropMetrics,
					MetricNames: map[string]bool{"metric": true},
				},
			},
			wantDimensionsMap: nil,
			wantError:         "",
		},
		{
			name: "drop_metrics_invalid",
			trs: []Rule{
				{
					Action: ActionDropMetrics,
				},
			},
			wantDimensionsMap: nil,
			wantError:         `field "metric_names" is required for "drop_metrics" translation rule`,
		},
		{
			name: "delta_metric_invalid",
			trs: []Rule{
				{
					Action: ActionDeltaMetric,
				},
			},
			wantError: `field "mapping" is required for "delta_metric" translation rule`,
		},
		{
			name: "drop_dimensions_invalid",
			trs: []Rule{
				{
					Action: ActionDropDimensions,
				},
			},
			wantError: `field "dimension_pairs" is required for "drop_dimensions" translation rule`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mt, err := NewMetricTranslator(tt.trs, 1, make(chan struct{}))
			if tt.wantError == "" {
				require.NoError(t, err)
				require.NotNil(t, mt)
				assert.Equal(t, tt.trs, mt.rules)
				assert.Equal(t, tt.wantDimensionsMap, mt.dimensionsMap)
			} else {
				require.Error(t, err)
				assert.Equal(t, tt.wantError, err.Error())
				require.Nil(t, mt)
			}
		})
	}
}

var (
	msec      = time.Now().Unix() * 1e3
	gaugeType = sfxpb.MetricType_GAUGE
)

func TestTranslateDataPoints(t *testing.T) {
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
			name: "rename_dimension_keys_filtered_metric",
			trs: []Rule{
				{
					Action: ActionRenameDimensionKeys,
					Mapping: map[string]string{
						"old.dimension": "new.dimension",
					},
					MetricNames: map[string]bool{
						"metric1": true,
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
					Dimensions: []*sfxpb.Dimension{
						{
							Key:   "old.dimension",
							Value: "value1",
						},
					},
				},
				{
					Metric:    "metric2",
					Timestamp: msec,
					Value: sfxpb.Datum{
						IntValue: generateIntPtr(13),
					},
					MetricType: &gaugeType,
					Dimensions: []*sfxpb.Dimension{
						{
							Key:   "old.dimension",
							Value: "value1",
						},
					},
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
					Dimensions: []*sfxpb.Dimension{
						{
							Key:   "new.dimension",
							Value: "value1",
						},
					},
				},
				// This metric doesn't match provided metric_name filter, so dimension key is not changed
				{
					Metric:    "metric2",
					Timestamp: msec,
					Value: sfxpb.Datum{
						IntValue: generateIntPtr(13),
					},
					MetricType: &gaugeType,
					Dimensions: []*sfxpb.Dimension{
						{
							Key:   "old.dimension",
							Value: "value1",
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
					Dimensions: []*sfxpb.Dimension{{Key: "dim1", Value: "val1"}},
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
					Dimensions: []*sfxpb.Dimension{{Key: "dim1", Value: "val1"}},
				},
			},
		},
		{
			name: "rename_metric and add dimensions",
			trs: []Rule{
				{
					Action: ActionRenameMetrics,
					Mapping: map[string]string{
						"k8s/container/mem/usage": "container_memory_usage_bytes",
					},
					AddDimensions: map[string]string{
						"dim2": "val2",
						"dim3": "val3",
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
					Dimensions: []*sfxpb.Dimension{
						{Key: "dim2", Value: "val2"},
						{Key: "dim3", Value: "val3"},
					},
				},
			},
		},
		{
			name: "rename_metric and add with existing dimension",
			trs: []Rule{
				{
					Action: ActionRenameMetrics,
					Mapping: map[string]string{
						"k8s/container/mem/usage": "container_memory_usage_bytes",
					},
					AddDimensions: map[string]string{
						"dim2": "val2",
						"dim3": "val3",
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
					Dimensions: []*sfxpb.Dimension{{Key: "dim1", Value: "val1"}},
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
					Dimensions: []*sfxpb.Dimension{
						{Key: "dim1", Value: "val1"},
						{Key: "dim2", Value: "val2"},
						{Key: "dim3", Value: "val3"},
					},
				},
			},
		},
		{
			name: "rename_metric with copy dimension",
			trs: []Rule{
				{
					Action: ActionRenameMetrics,
					Mapping: map[string]string{
						"k8s/container/mem/usage": "container_memory_usage_bytes",
					},
					CopyDimensions: map[string]string{
						"dim1": "copied1",
						"dim2": "copied2",
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
					Dimensions: []*sfxpb.Dimension{{Key: "dim1", Value: "val1"}},
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
					Dimensions: []*sfxpb.Dimension{
						{Key: "dim1", Value: "val1"},
						{Key: "copied1", Value: "val1"},
					},
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
							Value: "val2-alternate",
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
							Value: "val2-alternate",
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
					WithoutDimensions: []string{"dim3"},
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
						{
							Key:   "dim3",
							Value: "another",
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
					WithoutDimensions: []string{"dim2"},
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
					WithoutDimensions: []string{"dim2"},
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
						DoubleValue: generateFloatPtr(2.2),
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
		},
		{
			name: "aggregate_metric_mean",
			trs: []Rule{
				{
					Action:            ActionAggregateMetric,
					MetricName:        "metric1",
					WithoutDimensions: []string{"dim3"},
					AggregationMethod: "avg",
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
					},
				},
				{
					Metric:    "metric1",
					Timestamp: msec,
					Value: sfxpb.Datum{
						DoubleValue: generateFloatPtr(8.2),
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
						{
							Key:   "dim3",
							Value: "another",
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
					Metric:    "metric1",
					Timestamp: msec,
					Value: sfxpb.Datum{
						DoubleValue: generateFloatPtr(8.6),
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
					Metric:    "metric1",
					Timestamp: msec,
					Value: sfxpb.Datum{
						DoubleValue: generateFloatPtr(2.0),
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
			name: "drop_metrics",
			trs: []Rule{
				{
					Action: ActionDropMetrics,
					MetricNames: map[string]bool{
						"metric1": true,
						"metric2": true,
					},
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
				},
				{
					Metric:     "metric2",
					Timestamp:  msec,
					MetricType: &gaugeType,
					Value: sfxpb.Datum{
						DoubleValue: generateFloatPtr(2.2),
					},
				},
				{
					Metric:     "metric3",
					Timestamp:  msec,
					MetricType: &gaugeType,
					Value: sfxpb.Datum{
						DoubleValue: generateFloatPtr(2.2),
					},
				},
			},
			want: []*sfxpb.DataPoint{
				{
					Metric:     "metric3",
					Timestamp:  msec,
					MetricType: &gaugeType,
					Value: sfxpb.Datum{
						DoubleValue: generateFloatPtr(2.2),
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mt, err := NewMetricTranslator(tt.trs, 1, make(chan struct{}))
			require.NoError(t, err)
			assert.NotEqual(t, tt.want, tt.dps)
			got := mt.TranslateDataPoints(zap.NewNop(), tt.dps)
			assertEqualPoints(t, got, tt.want, tt.trs[0].Action)
		})
	}
}

func assertEqualPoints(t *testing.T, got []*sfxpb.DataPoint, want []*sfxpb.DataPoint, action Action) {
	// Sort metrics to handle not deterministic order from aggregation
	if action == ActionAggregateMetric {
		sort.Sort(byContent(want))
		sort.Sort(byContent(got))
	}

	if action == ActionRenameMetrics {
		for _, dp := range want {
			sort.Sort(byDimensions(dp.Dimensions))
		}

		for _, dp := range got {
			sort.Sort(byDimensions(dp.Dimensions))
		}
	}

	// Handle float values separately
	for i, dp := range got {
		if dp.GetValue().DoubleValue != nil {
			assert.InDelta(t, *want[i].GetValue().DoubleValue, *dp.GetValue().DoubleValue, 0.00000001)
			*dp.GetValue().DoubleValue = *want[i].GetValue().DoubleValue
		}
	}

	assert.Equal(t, want, got)
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
	}, 1, make(chan struct{}))
	require.NoError(t, err)

	assert.Equal(t, "new_dimension", mt.translateDimension("old_dimension"))
	assert.Equal(t, "new.dimension", mt.translateDimension("old.dimension"))
	assert.Equal(t, "another_dimension", mt.translateDimension("another_dimension"))

	// Test no rename_dimension_keys translation rule
	mt, err = NewMetricTranslator([]Rule{}, 1, make(chan struct{}))
	require.NoError(t, err)
	assert.Equal(t, "old_dimension", mt.translateDimension("old_dimension"))
}

func TestNewCalculateNewMetricErrors(t *testing.T) {
	tests := []struct {
		name    string
		metric1 string
		val1    *int64
		metric2 string
		val2    *int64
		wantErr string
	}{
		{
			name:    "operand1_nil",
			metric1: "foo",
			val1:    generateIntPtr(1),
			metric2: "metric2",
			val2:    generateIntPtr(1),
			wantErr: "",
		},
		{
			name:    "operand1_value_nil",
			metric1: "metric1",
			val1:    nil,
			metric2: "metric2",
			val2:    generateIntPtr(1),
			wantErr: "calculate_new_metric: operand1 has no numeric value",
		},
		{
			name:    "operand2_nil",
			metric1: "metric1",
			val1:    generateIntPtr(1),
			metric2: "foo",
			val2:    generateIntPtr(1),
			wantErr: "",
		},
		{
			name:    "operand2_value_nil",
			metric1: "metric1",
			val1:    generateIntPtr(1),
			metric2: "metric2",
			val2:    nil,
			wantErr: "calculate_new_metric: operand2 has no numeric value",
		},
		{
			name:    "divide_by_zero",
			metric1: "metric1",
			val1:    generateIntPtr(1),
			metric2: "metric2",
			val2:    generateIntPtr(0),
			wantErr: "calculate_new_metric: attempt to divide by zero, skipping",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			core, observedLogs := observer.New(zap.DebugLevel)
			logger := zap.New(core)
			dps := []*sfxpb.DataPoint{
				{
					Metric:     test.metric1,
					Timestamp:  msec,
					MetricType: &gaugeType,
					Value: sfxpb.Datum{
						IntValue: test.val1,
					},
					Dimensions: []*sfxpb.Dimension{{
						Key:   "dim1",
						Value: "val1",
					}},
				},
				{
					Metric:     test.metric2,
					Timestamp:  msec,
					MetricType: &gaugeType,
					Value: sfxpb.Datum{
						IntValue: test.val2,
					},
					Dimensions: []*sfxpb.Dimension{{
						Key:   "dim1",
						Value: "val1",
					}},
				},
			}
			mt, err := NewMetricTranslator([]Rule{{
				Action:         ActionCalculateNewMetric,
				MetricName:     "metric3",
				Operand1Metric: "metric1",
				Operand2Metric: "metric2",
				Operator:       MetricOperatorDivision,
			}}, 1, make(chan struct{}))
			require.NoError(t, err)
			tr := mt.TranslateDataPoints(logger, dps)
			require.Len(t, tr, 2)
			if test.wantErr == "" {
				require.Equal(t, 0, observedLogs.Len())
			} else {
				require.Equal(t, test.wantErr, observedLogs.All()[0].Message, "expected error: "+test.wantErr)
				require.Equal(t, 1, observedLogs.Len())
			}
		})
	}
}

func TestNewMetricTranslator_InvalidOperator(t *testing.T) {
	_, err := NewMetricTranslator([]Rule{{
		Action:         ActionCalculateNewMetric,
		MetricName:     "metric3",
		Operand1Metric: "metric1",
		Operand2Metric: "metric2",
		Operator:       "*",
	}}, 1, make(chan struct{}))
	require.Errorf(
		t,
		err,
		`invalid operator "*" for "calculate_new_metric" translation rule`,
	)
}

func TestCalcNewMetricInputPairs_SameDims(t *testing.T) {
	rule := Rule{
		Operand1Metric: "m1",
		Operand2Metric: "m2",
		Operator:       "/",
	}
	pts := []*sfxpb.DataPoint{
		{
			Metric:     "m1",
			Timestamp:  msec,
			MetricType: &gaugeType,
			Value: sfxpb.Datum{
				IntValue: generateIntPtr(84),
			},
			Dimensions: []*sfxpb.Dimension{{
				Key:   "dim1",
				Value: "val1",
			}},
		},
		{
			Metric:     "m2",
			Timestamp:  msec,
			MetricType: &gaugeType,
			Value: sfxpb.Datum{
				IntValue: generateIntPtr(48),
			},
			Dimensions: []*sfxpb.Dimension{{
				Key:   "dim1",
				Value: "val1",
			}},
		},
	}
	pairs := calcNewMetricInputPairs(pts, rule)
	require.Len(t, pairs, 1)
	pair := pairs[0]
	require.Equal(t, "m1", pair[0].Metric)
	require.Equal(t, "m2", pair[1].Metric)
}

func TestNewMetricInputPairs_MultiPairs(t *testing.T) {
	rule := Rule{
		Operand1Metric: "m1",
		Operand2Metric: "m2",
		Operator:       "/",
	}
	pts := []*sfxpb.DataPoint{
		{
			Metric:     "m1",
			Timestamp:  msec,
			MetricType: &gaugeType,
			Value: sfxpb.Datum{
				IntValue: generateIntPtr(1),
			},
			Dimensions: []*sfxpb.Dimension{{
				Key:   "dim1",
				Value: "val1",
			}},
		},
		{
			Metric:     "m2",
			Timestamp:  msec,
			MetricType: &gaugeType,
			Value: sfxpb.Datum{
				IntValue: generateIntPtr(2),
			},
			Dimensions: []*sfxpb.Dimension{{
				Key:   "dim1",
				Value: "val1",
			}},
		},
		{
			Metric:     "m1",
			Timestamp:  msec,
			MetricType: &gaugeType,
			Value: sfxpb.Datum{
				IntValue: generateIntPtr(3),
			},
			Dimensions: []*sfxpb.Dimension{{
				Key:   "dim1",
				Value: "val2",
			}},
		},
		{
			Metric:     "m2",
			Timestamp:  msec,
			MetricType: &gaugeType,
			Value: sfxpb.Datum{
				IntValue: generateIntPtr(4),
			},
			Dimensions: []*sfxpb.Dimension{{
				Key:   "dim1",
				Value: "val2",
			}},
		},
	}
	pairs := calcNewMetricInputPairs(pts, rule)
	require.Len(t, pairs, 2)
	pair1 := pairs[0]
	require.EqualValues(t, 1, *pair1[0].Value.IntValue)
	require.EqualValues(t, 2, *pair1[1].Value.IntValue)
	pair2 := pairs[1]
	require.EqualValues(t, 3, *pair2[0].Value.IntValue)
	require.EqualValues(t, 4, *pair2[1].Value.IntValue)
}

func TestCalcNewMetricInputPairs_DiffDims(t *testing.T) {
	rule := Rule{
		Operand1Metric: "m1",
		Operand2Metric: "m2",
		Operator:       "/",
	}
	pts := []*sfxpb.DataPoint{
		{
			Metric:     "m1",
			Timestamp:  msec,
			MetricType: &gaugeType,
			Value: sfxpb.Datum{
				IntValue: generateIntPtr(84),
			},
			Dimensions: []*sfxpb.Dimension{{
				Key:   "dim1",
				Value: "val1",
			}},
		},
		{
			Metric:     "m2",
			Timestamp:  msec,
			MetricType: &gaugeType,
			Value: sfxpb.Datum{
				IntValue: generateIntPtr(84),
			},
			Dimensions: []*sfxpb.Dimension{{
				Key:   "dim1",
				Value: "val2",
			}},
		},
	}
	pairs := calcNewMetricInputPairs(pts, rule)
	require.Nil(t, pairs)
}

func TestCalculateNewMetric_MatchingDims_Single(t *testing.T) {
	mt, err := NewMetricTranslator([]Rule{{
		Action:         ActionCalculateNewMetric,
		MetricName:     "metric3",
		Operand1Metric: "metric1",
		Operand2Metric: "metric2",
		Operator:       "/",
	}}, 1, make(chan struct{}))
	require.NoError(t, err)
	m1 := &sfxpb.DataPoint{
		Metric:     "metric1",
		Timestamp:  msec,
		MetricType: &gaugeType,
		Value: sfxpb.Datum{
			IntValue: generateIntPtr(1),
		},
		Dimensions: []*sfxpb.Dimension{{
			Key:   "dim1",
			Value: "val1",
		}},
	}
	m2 := &sfxpb.DataPoint{
		Metric:     "metric2",
		Timestamp:  msec,
		MetricType: &gaugeType,
		Value: sfxpb.Datum{
			IntValue: generateIntPtr(2),
		},
		Dimensions: []*sfxpb.Dimension{{
			Key:   "dim1",
			Value: "val1",
		}},
	}
	dps := []*sfxpb.DataPoint{m1, m2}
	translated := mt.TranslateDataPoints(zap.NewNop(), dps)
	m3 := &sfxpb.DataPoint{
		Metric:     "metric3",
		Timestamp:  msec,
		MetricType: &gaugeType,
		Value: sfxpb.Datum{
			DoubleValue: generateFloatPtr(0.5),
		},
		Dimensions: []*sfxpb.Dimension{{
			Key:   "dim1",
			Value: "val1",
		}},
	}
	want := []*sfxpb.DataPoint{m1, m2, m3}
	assertEqualPoints(t, translated, want, ActionCalculateNewMetric)
}

func TestCalculateNewMetric_MatchingDims_Multi(t *testing.T) {
	mt, err := NewMetricTranslator([]Rule{{
		Action:         ActionCalculateNewMetric,
		MetricName:     "metric3",
		Operand1Metric: "metric1",
		Operand2Metric: "metric2",
		Operator:       "/",
	}}, 1, make(chan struct{}))
	require.NoError(t, err)
	m1 := &sfxpb.DataPoint{
		Metric:     "metric1",
		Timestamp:  msec,
		MetricType: &gaugeType,
		Value: sfxpb.Datum{
			IntValue: generateIntPtr(1),
		},
		Dimensions: []*sfxpb.Dimension{{
			Key:   "dim1",
			Value: "val1",
		}},
	}
	m2 := &sfxpb.DataPoint{
		Metric:     "metric2",
		Timestamp:  msec,
		MetricType: &gaugeType,
		Value: sfxpb.Datum{
			IntValue: generateIntPtr(2),
		},
		Dimensions: []*sfxpb.Dimension{{
			Key:   "dim1",
			Value: "val1",
		}},
	}
	m1v2 := &sfxpb.DataPoint{
		Metric:     "metric1",
		Timestamp:  msec,
		MetricType: &gaugeType,
		Value: sfxpb.Datum{
			IntValue: generateIntPtr(1),
		},
		Dimensions: []*sfxpb.Dimension{{
			Key:   "dim1",
			Value: "val2",
		}},
	}
	m2v2 := &sfxpb.DataPoint{
		Metric:     "metric2",
		Timestamp:  msec,
		MetricType: &gaugeType,
		Value: sfxpb.Datum{
			IntValue: generateIntPtr(2),
		},
		Dimensions: []*sfxpb.Dimension{{
			Key:   "dim1",
			Value: "val2",
		}},
	}
	dps := []*sfxpb.DataPoint{m1, m2, m1v2, m2v2}
	translated := mt.TranslateDataPoints(zap.NewNop(), dps)
	m3 := &sfxpb.DataPoint{
		Metric:     "metric3",
		Timestamp:  msec,
		MetricType: &gaugeType,
		Value: sfxpb.Datum{
			DoubleValue: generateFloatPtr(0.5),
		},
		Dimensions: []*sfxpb.Dimension{{
			Key:   "dim1",
			Value: "val1",
		}},
	}
	m3v2 := &sfxpb.DataPoint{
		Metric:     "metric3",
		Timestamp:  msec,
		MetricType: &gaugeType,
		Value: sfxpb.Datum{
			DoubleValue: generateFloatPtr(0.5),
		},
		Dimensions: []*sfxpb.Dimension{{
			Key:   "dim1",
			Value: "val2",
		}},
	}
	want := []*sfxpb.DataPoint{m1, m2, m1v2, m2v2, m3, m3v2}
	assertEqualPoints(t, translated, want, ActionCalculateNewMetric)
}

func TestUnsupportedOperator(t *testing.T) {
	_, err := NewMetricTranslator([]Rule{{
		Action:         ActionCalculateNewMetric,
		MetricName:     "metric3",
		Operand1Metric: "metric1",
		Operand2Metric: "metric2",
		Operator:       "*",
	}}, 1, make(chan struct{}))
	require.Error(t, err)
}

func TestCalculateNewMetric_Double(t *testing.T) {
	mt, err := NewMetricTranslator([]Rule{{
		Action:         ActionCalculateNewMetric,
		MetricName:     "metric3",
		Operand1Metric: "metric1",
		Operand2Metric: "metric2",
		Operator:       "/",
	}}, 1, make(chan struct{}))
	require.NoError(t, err)
	m1 := &sfxpb.DataPoint{
		Metric:     "metric1",
		Timestamp:  msec,
		MetricType: &gaugeType,
		Value: sfxpb.Datum{
			DoubleValue: generateFloatPtr(1),
		},
		Dimensions: []*sfxpb.Dimension{{
			Key:   "dim1",
			Value: "val1",
		}},
	}
	m2 := &sfxpb.DataPoint{
		Metric:     "metric2",
		Timestamp:  msec,
		MetricType: &gaugeType,
		Value: sfxpb.Datum{
			DoubleValue: generateFloatPtr(2),
		},
		Dimensions: []*sfxpb.Dimension{{
			Key:   "dim1",
			Value: "val1",
		}},
	}
	dps := []*sfxpb.DataPoint{m1, m2}
	translated := mt.TranslateDataPoints(zap.NewNop(), dps)
	m3 := &sfxpb.DataPoint{
		Metric:     "metric3",
		Timestamp:  msec,
		MetricType: &gaugeType,
		Value: sfxpb.Datum{
			DoubleValue: generateFloatPtr(0.5),
		},
		Dimensions: []*sfxpb.Dimension{{
			Key:   "dim1",
			Value: "val1",
		}},
	}
	want := []*sfxpb.DataPoint{m1, m2, m3}
	assertEqualPoints(t, translated, want, ActionCalculateNewMetric)
}

func generateIntPtr(i int) *int64 {
	iPtr := int64(i)
	return &iPtr
}

func generateFloatPtr(i float64) *float64 {
	iPtr := i
	return &iPtr
}

func TestDimensionsEqual(t *testing.T) {
	tests := []struct {
		name   string
		d1, d2 []*sfxpb.Dimension
		want   bool
	}{
		{
			name: "eq_both_nil",
			d1:   nil,
			d2:   nil,
			want: true,
		},
		{
			name: "ne_one_nil",
			d1:   []*sfxpb.Dimension{{Key: "k0", Value: "v0"}},
			d2:   nil,
			want: false,
		},
		{
			name: "eq_one_dim",
			d1:   []*sfxpb.Dimension{{Key: "k0", Value: "v0"}},
			d2:   []*sfxpb.Dimension{{Key: "k0", Value: "v0"}},
			want: true,
		},
		{
			name: "ne_one_dim_val_mismatch",
			d1:   []*sfxpb.Dimension{{Key: "k0", Value: "v0"}},
			d2:   []*sfxpb.Dimension{{Key: "k0", Value: "x"}},
			want: false,
		},
		{
			name: "ne_one_dim_key_mismatch",
			d1:   []*sfxpb.Dimension{{Key: "k0", Value: "v0"}},
			d2:   []*sfxpb.Dimension{{Key: "x", Value: "v0"}},
			want: false,
		},
		{
			name: "eq_two_dims",
			d1:   []*sfxpb.Dimension{{Key: "k0", Value: "v0"}, {Key: "k1", Value: "v1"}},
			d2:   []*sfxpb.Dimension{{Key: "k0", Value: "v0"}, {Key: "k1", Value: "v1"}},
			want: true,
		},
		{
			name: "eq_two_dims_different_order",
			d1:   []*sfxpb.Dimension{{Key: "k0", Value: "v0"}, {Key: "k1", Value: "v1"}},
			d2:   []*sfxpb.Dimension{{Key: "k1", Value: "v1"}, {Key: "k0", Value: "v0"}},
			want: true,
		},
		{
			name: "eq_three_dims_different_order",
			d1:   []*sfxpb.Dimension{{Key: "k0", Value: "v0"}, {Key: "k1", Value: "v1"}, {Key: "k2", Value: "v2"}},
			d2:   []*sfxpb.Dimension{{Key: "k2", Value: "v2"}, {Key: "k0", Value: "v0"}, {Key: "k1", Value: "v1"}},
			want: true,
		},
		{
			name: "ne_first_two_dims_match",
			d1:   []*sfxpb.Dimension{{Key: "k0", Value: "v0"}, {Key: "k1", Value: "v1"}},
			d2:   []*sfxpb.Dimension{{Key: "k0", Value: "v0"}, {Key: "k1", Value: "v1"}, {Key: "k2", Value: "v2"}},
			want: false,
		},
		{
			name: "ne_first_two_dims_match_reversed",
			d1:   []*sfxpb.Dimension{{Key: "k0", Value: "v0"}, {Key: "k1", Value: "v1"}, {Key: "k2", Value: "v2"}},
			d2:   []*sfxpb.Dimension{{Key: "k0", Value: "v0"}, {Key: "k1", Value: "v1"}},
			want: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			b := dimensionsEqual(test.d1, test.d2)
			require.Equal(t, test.want, b)
		})
	}
}

func TestDeltaMetricDouble(t *testing.T) {
	const delta1 = 3
	const delta2 = 5
	md1 := doubleMD(10, 0)
	md2 := doubleMD(20, delta1)
	md3 := doubleMD(30, delta1+delta2)

	pts1, pts2 := requireDeltaMetricOk(t, md1, md2, md3)
	for _, pt := range pts1 {
		require.EqualValues(t, delta1, *pt.Value.DoubleValue)
	}
	for _, pt := range pts2 {
		require.EqualValues(t, delta2, *pt.Value.DoubleValue)
	}
}

func TestDeltaMetricInt(t *testing.T) {
	const delta1 = 7
	const delta2 = 11
	md1 := intMD(10, 0)
	md2 := intMD(20, delta1)
	md3 := intMD(30, delta1+delta2)
	pts1, pts2 := requireDeltaMetricOk(t, md1, md2, md3)
	for _, pt := range pts1 {
		require.EqualValues(t, delta1, *pt.Value.IntValue)
	}
	for _, pt := range pts2 {
		require.EqualValues(t, delta2, *pt.Value.IntValue)
	}
}

func TestNegativeDeltas(t *testing.T) {
	md1 := intMD(10, 0)
	md2 := intMD(20, 13)
	md3 := intMDAfterReset(30, 5)
	pts1, pts2 := requireDeltaMetricOk(t, md1, md2, md3)
	for _, pt := range pts1 {
		require.EqualValues(t, 13, *pt.Value.IntValue)
	}
	for _, pt := range pts2 {
		// since the counter went down (assuming a reset), we expect a delta of the most recent value
		require.EqualValues(t, 5, *pt.Value.IntValue)
	}
}

func TestDeltaTranslatorNoMatchingMapping(t *testing.T) {
	c := testConverter(t, map[string]string{"foo": "bar"})
	md := intMD(1, 1)
	idx := indexPts(c.MetricsToSignalFxV2(md))
	require.Len(t, idx, 1)
}

func TestDeltaTranslatorMismatchedValueTypes(t *testing.T) {
	c := testConverter(t, map[string]string{"system.cpu.time": "system.cpu.delta"})
	md1 := baseMD()
	intTS("cpu0", "user", 1, 1, 1, md1.SetEmptySum().DataPoints().AppendEmpty())

	_ = c.MetricsToSignalFxV2(wrapMetric(md1))
	md2 := baseMD()
	dblTS("cpu0", "user", 1, 1, 1, md2.SetEmptySum().DataPoints().AppendEmpty())
	pts := c.MetricsToSignalFxV2(wrapMetric(md2))
	idx := indexPts(pts)
	require.Len(t, idx, 1)
}

func requireDeltaMetricOk(t *testing.T, md1, md2, md3 pmetric.Metrics) (
	[]*sfxpb.DataPoint, []*sfxpb.DataPoint,
) {
	c := testConverter(t, map[string]string{"system.cpu.time": "system.cpu.delta"})

	dp1 := c.MetricsToSignalFxV2(md1)
	m1 := indexPts(dp1)
	require.Len(t, m1, 1)

	dp2 := c.MetricsToSignalFxV2(md2)
	m2 := indexPts(dp2)
	require.Len(t, m2, 2)

	origPts, ok := m2["system.cpu.time"]
	require.True(t, ok)

	deltaPts1, ok := m2["system.cpu.delta"]
	require.True(t, ok)
	require.Len(t, deltaPts1, len(origPts))
	counterType := sfxpb.MetricType_GAUGE
	for _, pt := range deltaPts1 {
		require.Equal(t, &counterType, pt.MetricType)
	}

	dp3 := c.MetricsToSignalFxV2(md3)
	m3 := indexPts(dp3)
	require.Len(t, m3, 2)

	deltaPts2, ok := m3["system.cpu.delta"]
	require.True(t, ok)
	require.Len(t, deltaPts2, len(origPts))
	for _, pt := range deltaPts2 {
		require.Equal(t, &counterType, pt.MetricType)
	}
	return deltaPts1, deltaPts2
}

func TestDropDimensions(t *testing.T) {
	tests := []struct {
		name        string
		rules       []Rule
		inputDps    []*sfxpb.DataPoint
		expectedDps []*sfxpb.DataPoint
	}{
		{
			name: "With metric name",
			rules: []Rule{
				{
					Action:     ActionDropDimensions,
					MetricName: "/metric.*/",
					MetricNames: map[string]bool{
						"testmetric": true,
					},
					DimensionPairs: map[string]map[string]bool{
						"dim_key1": nil,
						"dim_key2": {
							"dim_val1": true,
							"dim_val2": true,
						},
					},
				},
			},
			inputDps: []*sfxpb.DataPoint{
				{
					Metric: "metric1",
					Dimensions: []*sfxpb.Dimension{
						{
							Key:   "dim_key1",
							Value: "dim_val1",
						},
						{
							Key:   "dim_key2",
							Value: "dim_val1",
						},
					},
				},
				{
					Metric: "metrik1",
					Dimensions: []*sfxpb.Dimension{
						{
							Key:   "dim_key1",
							Value: "dim_val1",
						},
						{
							Key:   "dim_key2",
							Value: "dim_val1",
						},
					},
				},
				{
					Metric: "testmetric",
					Dimensions: []*sfxpb.Dimension{
						{
							Key:   "dim_key1",
							Value: "dim_val1",
						},
						{
							Key:   "dim_key2",
							Value: "dim_val1",
						},
					},
				},
			},
			expectedDps: []*sfxpb.DataPoint{
				{
					Metric:     "metric1",
					Dimensions: []*sfxpb.Dimension{},
				},
				{
					Metric: "metrik1",
					Dimensions: []*sfxpb.Dimension{
						{
							Key:   "dim_key1",
							Value: "dim_val1",
						},
						{
							Key:   "dim_key2",
							Value: "dim_val1",
						},
					},
				},
				{
					Metric:     "testmetric",
					Dimensions: []*sfxpb.Dimension{},
				},
			},
		},
		{
			name: "Without metric name",
			rules: []Rule{
				{
					Action: ActionDropDimensions,
					DimensionPairs: map[string]map[string]bool{
						"dim_key1": nil,
						"dim_key2": {
							"dim_val1": true,
							"dim_val2": true,
						},
					},
				},
			},
			inputDps: []*sfxpb.DataPoint{
				{
					Metric: "metric1",
					Dimensions: []*sfxpb.Dimension{
						{
							Key:   "dim_key1",
							Value: "dim_val1",
						},
						{
							Key:   "dim_key2",
							Value: "dim_val1",
						},
					},
				},
				{
					Metric: "metric2",
					Dimensions: []*sfxpb.Dimension{
						{
							Key:   "dim_key1",
							Value: "dim_val1",
						},
						{
							Key:   "dim_key2",
							Value: "dim_val1",
						},
					},
				},
				{
					Metric: "testmetric",
					Dimensions: []*sfxpb.Dimension{
						{
							Key:   "dim_key1",
							Value: "dim_val1",
						},
						{
							Key:   "dim_key2",
							Value: "dim_val1",
						},
					},
				},
			},
			expectedDps: []*sfxpb.DataPoint{
				{
					Metric:     "metric1",
					Dimensions: []*sfxpb.Dimension{},
				},
				{
					Metric:     "metric2",
					Dimensions: []*sfxpb.Dimension{},
				},
				{
					Metric:     "testmetric",
					Dimensions: []*sfxpb.Dimension{},
				},
			},
		},
		{
			name: "Drop dimension on all values",
			rules: []Rule{
				{
					Action: ActionDropDimensions,
					DimensionPairs: map[string]map[string]bool{
						"dim_key1": {},
						"dim_key2": nil,
					},
				},
			},
			inputDps: []*sfxpb.DataPoint{
				{
					Metric: "metric1",
					Dimensions: []*sfxpb.Dimension{
						{
							Key:   "dim_key1",
							Value: "dim_val1",
						},
						{
							Key:   "dim_key2",
							Value: "dim_val1",
						},
					},
				},
				{
					Metric: "metric2",
					Dimensions: []*sfxpb.Dimension{
						{
							Key:   "dim_key1",
							Value: "dim_val2",
						},
						{
							Key:   "dim_key2",
							Value: "dim_val2",
						},
					},
				},
			},
			expectedDps: []*sfxpb.DataPoint{
				{
					Metric:     "metric1",
					Dimensions: []*sfxpb.Dimension{},
				},
				{
					Metric:     "metric2",
					Dimensions: []*sfxpb.Dimension{},
				},
			},
		},
		{
			name: "Drop dimension on listed values",
			rules: []Rule{
				{
					Action: ActionDropDimensions,
					DimensionPairs: map[string]map[string]bool{
						"dim_key1": {"dim_val1": true},
						"dim_key2": {"dim_val2": true},
					},
				},
			},
			inputDps: []*sfxpb.DataPoint{
				{
					Metric: "metric1",
					Dimensions: []*sfxpb.Dimension{
						{
							Key:   "dim_key1",
							Value: "dim_val1",
						},
						{
							Key:   "dim_key2",
							Value: "dim_val1",
						},
					},
				},
				{
					Metric: "metric2",
					Dimensions: []*sfxpb.Dimension{
						{
							Key:   "dim_key1",
							Value: "dim_val2",
						},
						{
							Key:   "dim_key2",
							Value: "dim_val2",
						},
					},
				},
			},
			expectedDps: []*sfxpb.DataPoint{
				{
					Metric: "metric1",
					Dimensions: []*sfxpb.Dimension{
						{
							Key:   "dim_key2",
							Value: "dim_val1",
						},
					},
				},
				{
					Metric: "metric2",
					Dimensions: []*sfxpb.Dimension{
						{
							Key:   "dim_key1",
							Value: "dim_val2",
						},
					},
				},
			},
		},
		{
			name: "Do not drop dimension not listed",
			rules: []Rule{
				{
					Action: ActionDropDimensions,
					DimensionPairs: map[string]map[string]bool{
						"dim_key1": {"dim_val1": true},
						"dim_key2": {"dim_val2": true},
					},
				},
			},
			inputDps: []*sfxpb.DataPoint{
				{
					Metric: "metric1",
					Dimensions: []*sfxpb.Dimension{
						{
							Key:   "dim_key3",
							Value: "dim_val1",
						},
					},
				},
			},
			expectedDps: []*sfxpb.DataPoint{
				{
					Metric: "metric1",
					Dimensions: []*sfxpb.Dimension{
						{
							Key:   "dim_key3",
							Value: "dim_val1",
						},
					},
				},
			},
		},
		{
			name: "No op when dimensions do not exist on dp",
			rules: []Rule{
				{
					Action: ActionDropDimensions,
					DimensionPairs: map[string]map[string]bool{
						"dim_key1": {"dim_val1": true},
						"dim_key2": {"dim_val2": true},
					},
				},
			},
			inputDps: []*sfxpb.DataPoint{
				{
					Metric: "metric1",
				},
			},
			expectedDps: []*sfxpb.DataPoint{
				{
					Metric: "metric1",
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mt, err := NewMetricTranslator(test.rules, 1, make(chan struct{}))
			require.NoError(t, err)
			outputSFxDps := mt.TranslateDataPoints(zap.NewNop(), test.inputDps)
			require.Equal(t, test.expectedDps, outputSFxDps)
		})
	}
}

func TestDropDimensionsErrorCases(t *testing.T) {
	tests := []struct {
		name          string
		rules         []Rule
		expectedError string
	}{
		{
			name: "Test with invalid metric name pattern",
			rules: []Rule{
				{
					Action:     ActionDropDimensions,
					MetricName: "/metric.*(/",
					DimensionPairs: map[string]map[string]bool{
						"dim_key1": nil,
						"dim_key2": {
							"dim_val1": true,
							"dim_val2": true,
						},
					},
				},
			},
			expectedError: "failed creating metric matcher: error parsing regexp: missing closing ): `metric.*(`",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mt, err := NewMetricTranslator(test.rules, 1, make(chan struct{}))
			require.EqualError(t, err, test.expectedError)
			require.Nil(t, mt)
		})
	}
}

func testConverter(t *testing.T, mapping map[string]string) *MetricsConverter {
	rules := []Rule{{
		Action:  ActionDeltaMetric,
		Mapping: mapping,
	}}
	tr, err := NewMetricTranslator(rules, 1, make(chan struct{}))
	require.NoError(t, err)

	c, err := NewMetricsConverter(zap.NewNop(), tr, nil, nil, "", false, true)
	require.NoError(t, err)
	return c
}

func indexPts(pts []*sfxpb.DataPoint) map[string][]*sfxpb.DataPoint {
	m := map[string][]*sfxpb.DataPoint{}
	for _, pt := range pts {
		l := m[pt.Metric]
		m[pt.Metric] = append(l, pt)
	}
	return m
}

func doubleMD(secondsDelta int64, valueDelta float64) pmetric.Metrics {
	md := baseMD()
	ms := md.SetEmptySum()
	dblTS("cpu0", "user", secondsDelta, 100, valueDelta, ms.DataPoints().AppendEmpty())
	dblTS("cpu0", "system", secondsDelta, 200, valueDelta, ms.DataPoints().AppendEmpty())
	dblTS("cpu0", "idle", secondsDelta, 300, valueDelta, ms.DataPoints().AppendEmpty())
	dblTS("cpu1", "user", secondsDelta, 111, valueDelta, ms.DataPoints().AppendEmpty())
	dblTS("cpu1", "system", secondsDelta, 222, valueDelta, ms.DataPoints().AppendEmpty())
	dblTS("cpu1", "idle", secondsDelta, 333, valueDelta, ms.DataPoints().AppendEmpty())

	return wrapMetric(md)
}

func intMD(secondsDelta int64, valueDelta int64) pmetric.Metrics {
	md := baseMD()
	ms := md.SetEmptySum()
	intTS("cpu0", "user", secondsDelta, 100, valueDelta, ms.DataPoints().AppendEmpty())
	intTS("cpu0", "system", secondsDelta, 200, valueDelta, ms.DataPoints().AppendEmpty())
	intTS("cpu0", "idle", secondsDelta, 300, valueDelta, ms.DataPoints().AppendEmpty())
	intTS("cpu1", "user", secondsDelta, 111, valueDelta, ms.DataPoints().AppendEmpty())
	intTS("cpu1", "system", secondsDelta, 222, valueDelta, ms.DataPoints().AppendEmpty())
	intTS("cpu1", "idle", secondsDelta, 333, valueDelta, ms.DataPoints().AppendEmpty())

	return wrapMetric(md)
}

func intMDAfterReset(secondsDelta int64, valueDelta int64) pmetric.Metrics {
	md := baseMD()
	ms := md.SetEmptySum()
	intTS("cpu0", "user", secondsDelta, 0, valueDelta, ms.DataPoints().AppendEmpty())
	intTS("cpu0", "system", secondsDelta, 0, valueDelta, ms.DataPoints().AppendEmpty())
	intTS("cpu0", "idle", secondsDelta, 0, valueDelta, ms.DataPoints().AppendEmpty())
	intTS("cpu1", "user", secondsDelta, 0, valueDelta, ms.DataPoints().AppendEmpty())
	intTS("cpu1", "system", secondsDelta, 0, valueDelta, ms.DataPoints().AppendEmpty())
	intTS("cpu1", "idle", secondsDelta, 0, valueDelta, ms.DataPoints().AppendEmpty())

	return wrapMetric(md)
}

func baseMD() pmetric.Metric {
	out := pmetric.NewMetric()
	out.SetName("system.cpu.time")
	out.SetUnit("s")
	return out
}

func dblTS(lbl0 string, lbl1 string, secondsDelta int64, v float64, valueDelta float64, out pmetric.NumberDataPoint) {
	out.Attributes().PutStr("cpu", lbl0)
	out.Attributes().PutStr("state", lbl1)
	const startTime = 1600000000
	out.SetTimestamp(pcommon.Timestamp(time.Duration(startTime+secondsDelta) * time.Second))
	out.SetDoubleValue(v + valueDelta)
}

func intTS(lbl0 string, lbl1 string, secondsDelta int64, v int64, valueDelta int64, out pmetric.NumberDataPoint) {
	out.Attributes().PutStr("cpu", lbl0)
	out.Attributes().PutStr("state", lbl1)
	const startTime = 1600000000
	out.SetTimestamp(pcommon.Timestamp(time.Duration(startTime+secondsDelta) * time.Second))
	out.SetIntValue(v + valueDelta)
}

func wrapMetric(m pmetric.Metric) pmetric.Metrics {
	out := pmetric.NewMetrics()
	m.CopyTo(out.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty())
	return out
}
