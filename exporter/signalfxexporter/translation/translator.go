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
)

// Action is the enum to capture actions to perform on metrics.
type Action string

const (
	// Action_RENAME_DIMENSION_KEYS renames dimension keys using TranslationRule.Mapping.
	Action_RENAME_DIMENSION_KEYS Action = "rename_dimension_keys"

	// Action_RENAME_METRICS renames metrics using TranslationRule.Mapping.
	Action_RENAME_METRICS Action = "rename_metrics"

	// Action_MULTIPLY_INT scales integer metrics by multiplying their values using
	// TranslationRule.ScaleFactorsInt key/values as metric_name/multiplying_factor
	Action_MULTIPLY_INT Action = "multiply_int"

	// Action_DIVIDE_INT scales integer metric by dividing their values using
	// TranslationRule.ScaleFactorsInt key/values as metric_name/divisor
	Action_DIVIDE_INT Action = "divide_int"

	// Action_MULTIPLY_FLOAT scales integer metric by dividing their values using
	// TranslationRule.ScaleFactorsFloat key/values as metric_name/multiplying_factor
	Action_MULTIPLY_FLOAT Action = "multiply_float"

	// Action_COPY_METRICS copies metrics using TranslationRule.From, TranslationRule.To and
	// TranslationRule.DimensionsFilter.
	Action_COPY_METRICS Action = "copy_metrics"

	// Action_SPLIT_METRIC splits a metric with TranslationRule.MetricName into multiple metrics
	// based on a dimension specified in TranslationRule.DimensionKey.
	// TranslationRule.Mapping represents "dimension value" -> "new metric name" for the translation.
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
	Action_SPLIT_METRIC Action = "split_metric"
)

type TranslationRule struct {
	// Action specifies the translation action to be applied on metric
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
}

type MetricTranslator struct {
	translationRules []TranslationRule

	// Additional map to be used only for dimension renaming in metadata
	dimensionsMap map[string]string
}

func NewMetricTranslator(translationRules []TranslationRule) (*MetricTranslator, error) {
	err := validateTranslationRules(translationRules)
	if err != nil {
		return nil, err
	}

	return &MetricTranslator{
		translationRules: translationRules,
		dimensionsMap:    createDimensionsMap(translationRules),
	}, nil
}

func validateTranslationRules(translationRules []TranslationRule) error {
	var renameDimentionKeysFound bool
	for _, tr := range translationRules {
		switch tr.Action {
		case Action_RENAME_DIMENSION_KEYS:
			if tr.Mapping == nil {
				return fmt.Errorf("Field \"mapping\" is required for %q translation rule", tr.Action)
			}
			if renameDimentionKeysFound {
				return fmt.Errorf("Only one %q translation rule can be specified", tr.Action)
			}
			renameDimentionKeysFound = true
		case Action_RENAME_METRICS:
			if tr.Mapping == nil {
				return fmt.Errorf("Field \"mapping\" is required for %q translation rule", tr.Action)
			}
		case Action_MULTIPLY_INT, Action_DIVIDE_INT:
			if tr.ScaleFactorsInt == nil {
				return fmt.Errorf("Field \"scale_factors_int\" is required for %q translation rule", tr.Action)
			}
		case Action_MULTIPLY_FLOAT:
			if tr.ScaleFactorsFloat == nil {
				return fmt.Errorf("Field \"scale_factors_float\" is required for %q translation rule", tr.Action)
			}
		case Action_COPY_METRICS:
			if tr.Mapping == nil {
				return fmt.Errorf("Field \"mapping\" is required for %q translation rule", tr.Action)
			}
		case Action_SPLIT_METRIC:
			if tr.MetricName == "" || tr.DimensionKey == "" || tr.Mapping == nil {
				return fmt.Errorf(
					"Fields \"metric_name\", \"dimension_key\", and \"mapping\" are required for %q translation rule",
					tr.Action)
			}

		default:
			return fmt.Errorf("Unknown \"action\" value: %q", tr.Action)
		}
	}
	return nil
}

// createDimensionsMap creates an additional map for dimensions
// from Action_RENAME_DIMENSION_KEYS actions in translationRules.
func createDimensionsMap(translationRules []TranslationRule) map[string]string {
	for _, tr := range translationRules {
		if tr.Action == Action_RENAME_DIMENSION_KEYS {
			return tr.Mapping
		}
	}

	return nil
}

func (mp *MetricTranslator) TranslateDataPoints(sfxDataPoints []*sfxpb.DataPoint) []*sfxpb.DataPoint {
	processedDataPoints := sfxDataPoints

	for _, tr := range mp.translationRules {
		newDataPoints := []*sfxpb.DataPoint{}

		for _, dp := range processedDataPoints {
			switch tr.Action {
			case Action_RENAME_DIMENSION_KEYS:
				for _, d := range dp.Dimensions {
					if newKey, ok := tr.Mapping[d.Key]; ok {
						d.Key = newKey
					}
				}
			case Action_RENAME_METRICS:
				if newKey, ok := tr.Mapping[dp.Metric]; ok {
					dp.Metric = newKey
				}
			case Action_MULTIPLY_INT:
				if multiplier, ok := tr.ScaleFactorsInt[dp.Metric]; ok {
					v := dp.GetValue().IntValue
					if v != nil {
						*v = *v * multiplier
					}
				}
			case Action_DIVIDE_INT:
				if divisor, ok := tr.ScaleFactorsInt[dp.Metric]; ok {
					v := dp.GetValue().IntValue
					if v != nil {
						*v = *v / divisor
					}
				}
			case Action_MULTIPLY_FLOAT:
				if multiplier, ok := tr.ScaleFactorsFloat[dp.Metric]; ok {
					v := dp.GetValue().DoubleValue
					if v != nil {
						*v = *v * multiplier
					}
				}
			case Action_COPY_METRICS:
				if newMetric, ok := tr.Mapping[dp.Metric]; ok {
					newDataPoint := proto.Clone(dp).(*sfxpb.DataPoint)
					newDataPoint.Metric = newMetric
					newDataPoints = append(newDataPoints, newDataPoint)
				}
			case Action_SPLIT_METRIC:
				if tr.MetricName == dp.Metric {
					splitMetric(dp, tr.DimensionKey, tr.Mapping)
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
