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
	"fmt"

	"github.com/gogo/protobuf/proto"
	sfxpb "github.com/signalfx/com_signalfx_metrics_protobuf/model"
	"go.uber.org/zap"
)

// Action is the enum to capture actions to perform on metrics.
type Action string

const (
	// ActionRenameDimensionKeys renames dimension keys using Rule.Mapping.
	ActionRenameDimensionKeys Action = "rename_dimension_keys"

	// ActionRenameMetrics renames metrics using Rule.Mapping.
	ActionRenameMetrics Action = "rename_metrics"

	// ActionMultiplyInt scales integer metrics by multiplying their values using
	// Rule.ScaleFactorsInt key/values as metric_name/multiplying_factor
	ActionMultiplyInt Action = "multiply_int"

	// ActionDivideInt scales integer metric by dividing their values using
	// Rule.ScaleFactorsInt key/values as metric_name/divisor
	ActionDivideInt Action = "divide_int"

	// ActionMultiplyFloat scales integer metric by dividing their values using
	// Rule.ScaleFactorsFloat key/values as metric_name/multiplying_factor
	ActionMultiplyFloat Action = "multiply_float"

	// ActionConvertValues converts float metrics values to integer values using
	// Rule.TypesMapping key/values as metric_name/new_type.
	ActionConvertValues Action = "convert_values"

	// ActionCopyMetrics copies metrics using Rule.Mapping
	ActionCopyMetrics Action = "copy_metrics"

	// ActionSplitMetric splits a metric with Rule.MetricName into multiple metrics
	// based on a dimension specified in Rule.DimensionKey.
	// Rule.Mapping represents "dimension value" -> "new metric name" for the translation.
	// For example, having the following translation rule:
	//   - action: split_metric
	// 	   metric_name: k8s.pod.network.io
	//     dimension_key: direction
	//     mapping:
	//       receive: pod_network_receive_bytes_total
	//       transmit: pod_network_transmit_bytes_total
	// The following translations will be performed:
	// k8s.pod.network.io{direction="receive"} -> pod_network_receive_bytes_total{}
	// k8s.pod.network.io{direction="transmit"} -> pod_network_transmit_bytes_total{}
	ActionSplitMetric Action = "split_metric"
)

// MetricValueType is the enum to capture valid metric value types that can be converted
type MetricValueType string

const (
	// MetricValueTypeInt represents integer metric value type
	MetricValueTypeInt MetricValueType = "int"
	// MetricValueTypeDouble represents double metric value type
	MetricValueTypeDouble MetricValueType = "double"
)

type Rule struct {
	// Action specifies the translation action to be applied on metrics.
	// This is a required field.
	Action Action `mapstructure:"action"`

	// Mapping specifies key/value mapping that is used by rename_dimension_keys,
	// rename_metrics, copy_metrics, and split_metric actions.
	Mapping map[string]string `mapstructure:"mapping"`

	// ScaleFactorsInt is used by multiply_int and divide_int action to scale
	// integer metric values, key/value format: metric_name/scale_factor
	ScaleFactorsInt map[string]int64 `mapstructure:"scale_factors_int"`

	// ScaleFactorsInt is used by multiply_float action to scale
	// float metric values, key/value format: metric_name/scale_factor
	ScaleFactorsFloat map[string]float64 `mapstructure:"scale_factors_float"`

	// MetricName is used by "split_metric" translation rule to specify a name
	// of a metric that will be split.
	MetricName string `mapstructure:"metric_name"`
	// DimensionKey is used by "split_metric" translation rule action to specify dimension key
	// that will be used to translate the metric datapoints. Datapoints that don't have
	// the specified dimension key will not be translated.
	DimensionKey string `mapstructure:"dimension_key"`

	// TypesMapping is represents metric_name/metric_type key/value pairs,
	// used by ActionConvertValues.
	TypesMapping map[string]MetricValueType `mapstructure:"types_mapping"`
}

type MetricTranslator struct {
	rules []Rule

	// Additional map to be used only for dimension renaming in metadata
	dimensionsMap map[string]string
}

func NewMetricTranslator(rules []Rule) (*MetricTranslator, error) {
	err := validateTranslationRules(rules)
	if err != nil {
		return nil, err
	}

	return &MetricTranslator{
		rules:         rules,
		dimensionsMap: createDimensionsMap(rules),
	}, nil
}

func validateTranslationRules(rules []Rule) error {
	var renameDimentionKeysFound bool
	for _, tr := range rules {
		switch tr.Action {
		case ActionRenameDimensionKeys:
			if tr.Mapping == nil {
				return fmt.Errorf("field \"mapping\" is required for %q translation rule", tr.Action)
			}
			if renameDimentionKeysFound {
				return fmt.Errorf("only one %q translation rule can be specified", tr.Action)
			}
			renameDimentionKeysFound = true
		case ActionRenameMetrics:
			if tr.Mapping == nil {
				return fmt.Errorf("field \"mapping\" is required for %q translation rule", tr.Action)
			}
		case ActionMultiplyInt:
			if tr.ScaleFactorsInt == nil {
				return fmt.Errorf("field \"scale_factors_int\" is required for %q translation rule", tr.Action)
			}
		case ActionDivideInt:
			if tr.ScaleFactorsInt == nil {
				return fmt.Errorf("field \"scale_factors_int\" is required for %q translation rule", tr.Action)
			}
			for k, v := range tr.ScaleFactorsInt {
				if v == 0 {
					return fmt.Errorf("\"scale_factors_int\" for %q translation rule has 0 value for %q metric", tr.Action, k)
				}
			}
		case ActionMultiplyFloat:
			if tr.ScaleFactorsFloat == nil {
				return fmt.Errorf("field \"scale_factors_float\" is required for %q translation rule", tr.Action)
			}
		case ActionCopyMetrics:
			if tr.Mapping == nil {
				return fmt.Errorf("field \"mapping\" is required for %q translation rule", tr.Action)
			}
		case ActionSplitMetric:
			if tr.MetricName == "" || tr.DimensionKey == "" || tr.Mapping == nil {
				return fmt.Errorf(
					"fields \"metric_name\", \"dimension_key\", and \"mapping\" are required for %q translation rule",
					tr.Action)
			}
		case ActionConvertValues:
			if tr.TypesMapping == nil {
				return fmt.Errorf("field \"types_mapping\" are required for %q translation rule", tr.Action)
			}
			for k, v := range tr.TypesMapping {
				if v != MetricValueTypeInt && v != MetricValueTypeDouble {
					return fmt.Errorf("invalid value type %q set for metric %q in \"types_mapping\"", v, k)
				}
			}
		default:
			return fmt.Errorf("unknown \"action\" value: %q", tr.Action)
		}
	}
	return nil
}

// createDimensionsMap creates an additional map for dimensions
// from ActionRenameDimensionKeys actions in rules.
func createDimensionsMap(rules []Rule) map[string]string {
	for _, tr := range rules {
		if tr.Action == ActionRenameDimensionKeys {
			return tr.Mapping
		}
	}

	return nil
}

func (mp *MetricTranslator) TranslateDataPoints(logger *zap.Logger, sfxDataPoints []*sfxpb.DataPoint) []*sfxpb.DataPoint {
	processedDataPoints := sfxDataPoints

	for _, tr := range mp.rules {
		newDataPoints := []*sfxpb.DataPoint{}

		for _, dp := range processedDataPoints {
			switch tr.Action {
			case ActionRenameDimensionKeys:
				for _, d := range dp.Dimensions {
					if newKey, ok := tr.Mapping[d.Key]; ok {
						d.Key = newKey
					}
				}
			case ActionRenameMetrics:
				if newKey, ok := tr.Mapping[dp.Metric]; ok {
					dp.Metric = newKey
				}
			case ActionMultiplyInt:
				if multiplier, ok := tr.ScaleFactorsInt[dp.Metric]; ok {
					v := dp.GetValue().IntValue
					if v != nil {
						*v = *v * multiplier
					}
				}
			case ActionDivideInt:
				if divisor, ok := tr.ScaleFactorsInt[dp.Metric]; ok {
					v := dp.GetValue().IntValue
					if v != nil {
						*v = *v / divisor
					}
				}
			case ActionMultiplyFloat:
				if multiplier, ok := tr.ScaleFactorsFloat[dp.Metric]; ok {
					v := dp.GetValue().DoubleValue
					if v != nil {
						*v = *v * multiplier
					}
				}
			case ActionCopyMetrics:
				if newMetric, ok := tr.Mapping[dp.Metric]; ok {
					newDataPoint := proto.Clone(dp).(*sfxpb.DataPoint)
					newDataPoint.Metric = newMetric
					newDataPoints = append(newDataPoints, newDataPoint)
				}
			case ActionSplitMetric:
				if tr.MetricName == dp.Metric {
					splitMetric(dp, tr.DimensionKey, tr.Mapping)
				}
			case ActionConvertValues:
				if newType, ok := tr.TypesMapping[dp.Metric]; ok {
					convertMetricValue(logger, dp, newType)
				}
			}
		}

		processedDataPoints = append(processedDataPoints, newDataPoints...)
	}

	return processedDataPoints
}

func (mp *MetricTranslator) TranslateDimension(orig string) string {
	if translated, ok := mp.dimensionsMap[orig]; ok {
		return translated
	}
	return orig
}

// splitMetric renames a metric with "dimension key" == dimensionKey to mapping["dimension value"],
// datapoint not changed if not dimension found equal to dimensionKey:mapping->key.
func splitMetric(dp *sfxpb.DataPoint, dimensionKey string, mapping map[string]string) {
	if len(dp.Dimensions) == 0 {
		return
	}

	dimensions := make([]*sfxpb.Dimension, 0, len(dp.Dimensions)-1)
	var match bool
	for i, d := range dp.Dimensions {
		if dimensionKey == d.Key {
			if newName, ok := mapping[d.Value]; ok {
				// The dimension value matches the mapping, proceeding
				dp.Metric = newName
				match = true
				continue
			}
			// The dimension value doesn't match the mapping, keep the datapoint as is
			return
		}

		// No dimension key found for the specified dimensionKey, keep the datapoint as is
		if i == len(dp.Dimensions)-1 && !match {
			return
		}

		dimensions = append(dimensions, d)
	}

	dp.Dimensions = dimensions
}

func convertMetricValue(logger *zap.Logger, dp *sfxpb.DataPoint, newType MetricValueType) {
	switch newType {
	case MetricValueTypeInt:
		val := dp.GetValue().DoubleValue
		if val == nil {
			logger.Debug("only datapoint of \"double\" type can be converted to int",
				zap.String("metric", dp.Metric))
			return
		}
		var intVal = int64(*val)
		dp.Value = sfxpb.Datum{IntValue: &intVal}
	case MetricValueTypeDouble:
		val := dp.GetValue().IntValue
		if val == nil {
			logger.Debug("only datapoint of \"int\" type can be converted to double",
				zap.String("metric", dp.Metric))
			return
		}
		var floatVal = float64(*val)
		dp.Value = sfxpb.Datum{DoubleValue: &floatVal}
	}
}
