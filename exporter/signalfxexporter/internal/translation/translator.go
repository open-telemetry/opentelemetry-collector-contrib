// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translation // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/translation"

import (
	"fmt"
	"sort"
	"strings"

	"github.com/gogo/protobuf/proto"
	sfxpb "github.com/signalfx/com_signalfx_metrics_protobuf/model"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/translation/dpfilters"
)

// Action is the enum to capture actions to perform on metrics.
type Action string

const (
	// ActionRenameDimensionKeys renames dimension keys using Rule.Mapping.
	// The rule can be applied only to particular metrics if MetricNames are provided,
	// otherwise applied to all metrics.
	ActionRenameDimensionKeys Action = "rename_dimension_keys"

	// ActionRenameMetrics renames metrics using Rule.Mapping.
	ActionRenameMetrics Action = "rename_metrics"

	// ActionMultiplyInt scales integer metrics by multiplying their values using
	// Rule.ScaleFactorsInt key/values as metric_name/multiplying_factor
	ActionMultiplyInt Action = "multiply_int"

	// ActionDivideInt scales integer metric by dividing their values using
	// Rule.ScaleFactorsInt key/values as metric_name/divisor
	ActionDivideInt Action = "divide_int"

	// ActionMultiplyFloat scales integer metric by multiplying their values using
	// Rule.ScaleFactorsFloat key/values as metric_name/multiplying_factor
	// This rule can only be applied to metrics that are a float value
	ActionMultiplyFloat Action = "multiply_float"

	// ActionConvertValues converts metric values from int to float or float to int
	// Rule.TypesMapping key/values as metric_name/new_type.
	ActionConvertValues Action = "convert_values"

	// ActionCopyMetrics copies metrics using Rule.Mapping.
	// Rule.DimensionKey and Rule.DimensionValues can be used to filter datapoints that must be copied,
	// if these fields are set, only metics having a dimension with key == Rule.DimensionKey and
	// value in Rule.DimensionValues will be copied.
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

	// ActionAggregateMetric aggregates metrics excluding dimensions set in tr.WithoutDimensions.
	// This method is equivalent of "without" clause in Prometheus aggregation:
	// https://prometheus.io/docs/prometheus/latest/querying/operators/#aggregation-operators
	// It takes datapoints with name tr.MetricName and aggregates them to a smaller set keeping the same name.
	// It drops the dimensions provided in tr.WithoutDimensions and keeps others as is.
	// tr.AggregationMethod is used to specify a method to aggregate the values.
	// For example, having the following translation rule:
	// - action: aggregate_metric
	//   metric_name: machine_cpu_cores
	//   aggregation_method: count
	//   without_dimensions:
	//   - cpu
	// The following translations will be performed:
	// Original datapoints:
	//   machine_cpu_cores{cpu="cpu1",host="host1"} 0.22
	//   machine_cpu_cores{cpu="cpu2",host="host1"} 0.11
	//   machine_cpu_cores{cpu="cpu1",host="host2"} 0.33
	// Transformed datapoints:
	//   machine_cpu_cores{host="host1"} 2
	//   machine_cpu_cores{host="host2"} 1
	ActionAggregateMetric Action = "aggregate_metric"

	// ActionCalculateNewMetric calculates a new metric based on two existing metrics.
	// It takes two operand metrics, an operator, and a metric name and produces a new metric with the given
	// metric name, but with the attributes of the first operand metric.
	// For example, for the following translation rule:
	// - action: calculate_new_metric
	//  metric_name: memory.utilization
	//  operand1_metric: memory.used
	//  operand2_metric: memory.total
	//  operator: /
	// the integer value of the 'memory.used' metric will be divided by the integer value of 'memory.total'. The
	// result will be a new float metric with the name 'memory.utilization' and the value of the quotient. The
	// new metric will also get any attributes of the 'memory.used' metric except for its value and metric name.
	// Currently only integer inputs are handled and only division is supported.
	ActionCalculateNewMetric Action = "calculate_new_metric"

	// ActionDropMetrics drops datapoints with metric name defined in "metric_names".
	ActionDropMetrics Action = "drop_metrics"

	// ActionDeltaMetric creates a new delta (cumulative) metric from an existing non-cumulative int or double
	// metric. It takes mappings of names of the existing metrics to the names of the new, delta metrics to be
	// created. All dimensions will be preserved.
	ActionDeltaMetric Action = "delta_metric"

	// ActionDropDimensions drops specified dimensions. If no corresponding metric names are provided, the
	// dimensions are dropped globally from all datapoints. If dimension values are provided, only datapoints
	// with matching dimension values are dropped. Below are the possible configurations.
	// - action: drop_dimensions
	//   metric_names:
	//     k8s.pod.phase: true
	//   dimension_pairs:
	//     dim_key1:
	//     dim_key2:
	//       dim_val1: true
	//       dim_val2: true
	// - action: drop_dimensions
	//   dimension_pairs:
	//     dim_key1:
	ActionDropDimensions Action = "drop_dimensions"
)

type MetricOperator string

const (
	// MetricOperatorDivision is the MetricOperator division.
	MetricOperatorDivision MetricOperator = "/"
)

// MetricValueType is the enum to capture valid metric value types that can be converted
type MetricValueType string

const (
	// MetricValueTypeInt represents integer metric value type
	MetricValueTypeInt MetricValueType = "int"
	// MetricValueTypeDouble represents double metric value type
	MetricValueTypeDouble MetricValueType = "double"
)

// AggregationMethod is the enum used to capture aggregation method
type AggregationMethod string

// Values for enum AggregationMethodCount.
const (
	AggregationMethodCount AggregationMethod = "count"
	AggregationMethodAvg   AggregationMethod = "avg"
	AggregationMethodSum   AggregationMethod = "sum"
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
	// DimensionKey is also used by "copy_metrics" for filterring.
	DimensionKey string `mapstructure:"dimension_key"`

	// DimensionValues is used by "copy_metrics" to filter out datapoints with dimensions values
	// not matching values set in this field
	DimensionValues map[string]bool `mapstructure:"dimension_values"`

	// TypesMapping is represents metric_name/metric_type key/value pairs,
	// used by ActionConvertValues.
	TypesMapping map[string]MetricValueType `mapstructure:"types_mapping"`

	// AggregationMethod specifies method used by "aggregate_metric" translation rule
	AggregationMethod AggregationMethod `mapstructure:"aggregation_method"`

	// WithoutDimensions used by "aggregate_metric" translation rule to specify dimensions to be
	// excluded by aggregation.
	WithoutDimensions []string `mapstructure:"without_dimensions"`

	// AddDimensions used by "rename_metrics" translation rule to add dimensions that are necessary for
	// existing SFx content for desired metric name
	AddDimensions map[string]string `mapstructure:"add_dimensions"`

	// CopyDimensions used by "rename_metrics" translation rule to copy dimensions that are necessary for
	// existing SFx content for desired metric name.  This will duplicate the dimension value and isn't a rename.
	CopyDimensions map[string]string `mapstructure:"copy_dimensions"`

	// MetricNames is used by "rename_dimension_keys" and "drop_metrics" translation rules.
	MetricNames map[string]bool `mapstructure:"metric_names"`

	Operand1Metric string         `mapstructure:"operand1_metric"`
	Operand2Metric string         `mapstructure:"operand2_metric"`
	Operator       MetricOperator `mapstructure:"operator"`

	// DimensionPairs used by "drop_dimensions" translation rule to specify dimension pairs that
	// should be dropped.
	DimensionPairs map[string]map[string]bool `mapstructure:"dimension_pairs"`

	metricMatcher *dpfilters.StringFilter
}

type MetricTranslator struct {
	rules []Rule

	// Additional map to be used only for dimension renaming in metadata
	dimensionsMap map[string]string

	deltaTranslator *deltaTranslator
}

func NewMetricTranslator(rules []Rule, ttl int64) (*MetricTranslator, error) {
	err := validateTranslationRules(rules)
	if err != nil {
		return nil, err
	}

	err = processRules(rules)
	if err != nil {
		return nil, err
	}

	return &MetricTranslator{
		rules:           rules,
		dimensionsMap:   createDimensionsMap(rules),
		deltaTranslator: newDeltaTranslator(ttl),
	}, nil
}

func validateTranslationRules(rules []Rule) error {
	var renameDimensionKeysFound bool
	for _, tr := range rules {
		switch tr.Action {
		case ActionRenameDimensionKeys:
			if tr.Mapping == nil {
				return fmt.Errorf("field \"mapping\" is required for %q translation rule", tr.Action)
			}
			if len(tr.MetricNames) == 0 {
				if renameDimensionKeysFound {
					return fmt.Errorf("only one %q translation rule without \"metric_names\" can be specified", tr.Action)
				}
				renameDimensionKeysFound = true
			}
		case ActionRenameMetrics:
			if tr.Mapping == nil {
				return fmt.Errorf("field \"mapping\" is required for %q translation rule", tr.Action)
			}
			if tr.CopyDimensions != nil {
				for k, v := range tr.CopyDimensions {
					if k == "" || v == "" {
						return fmt.Errorf("mapping \"copy_dimensions\" for %q translation rule must not contain empty string keys or values", tr.Action)
					}
				}
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
			if tr.DimensionKey != "" && len(tr.DimensionValues) == 0 {
				return fmt.Errorf(
					"\"dimension_values_filer\" has to be provided if \"dimension_key\" is set for %q translation rule",
					tr.Action)
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
		case ActionAggregateMetric:
			if tr.MetricName == "" || tr.AggregationMethod == "" || len(tr.WithoutDimensions) == 0 {
				return fmt.Errorf("fields \"metric_name\", \"without_dimensions\", and \"aggregation_method\" "+
					"are required for %q translation rule", tr.Action)
			}
			if tr.AggregationMethod != AggregationMethodCount &&
				tr.AggregationMethod != AggregationMethodSum &&
				tr.AggregationMethod != AggregationMethodAvg {
				return fmt.Errorf("invalid \"aggregation_method\": %q provided for %q translation rule",
					tr.AggregationMethod, tr.Action)
			}
		case ActionCalculateNewMetric:
			if tr.MetricName == "" || tr.Operand1Metric == "" || tr.Operand2Metric == "" || tr.Operator == "" {
				return fmt.Errorf(`fields "metric_name", "operand1_metric", "operand2_metric", and "operator" are `+
					"required for %q translation rule", tr.Action)
			}
			if tr.Operator != MetricOperatorDivision {
				return fmt.Errorf("invalid operator %q for %q translation rule", tr.Operator, tr.Action)
			}
		case ActionDropMetrics:
			if len(tr.MetricNames) == 0 {
				return fmt.Errorf(`field "metric_names" is required for %q translation rule`, tr.Action)
			}
		case ActionDeltaMetric:
			if len(tr.Mapping) == 0 {
				return fmt.Errorf(`field "mapping" is required for %q translation rule`, tr.Action)
			}
		case ActionDropDimensions:
			if len(tr.DimensionPairs) == 0 {
				return fmt.Errorf(`field "dimension_pairs" is required for %q translation rule`, tr.Action)
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

func processRules(rules []Rule) error {
	for i, tr := range rules {
		if tr.Action == ActionDropDimensions {
			// Set metric name filter, if metric name(s) are specified on the rule.
			// When "drop_dimensions" actions is not scoped to a metric name, the
			// specified dimensions will be globally dropped from all datapoints
			// irrespective of metric name.
			if metricNames := getMetricNamesAsSlice(tr.MetricName, tr.MetricNames); len(metricNames) > 0 {
				metricMatcher, err := dpfilters.NewStringFilter(metricNames)
				if err != nil {
					return fmt.Errorf("failed creating metric matcher: %w", err)
				}
				rules[i].metricMatcher = metricMatcher
			}
		}
	}
	return nil
}

// getMetricNamesAsSlice returns a slice of metric names consolidating entries from metricName string
// and metricNames set.
func getMetricNamesAsSlice(metricName string, metricNames map[string]bool) []string {
	out := make([]string, 0, len(metricNames)+1)
	for m := range metricNames {
		out = append(out, m)
	}
	if metricName != "" {
		out = append(out, metricName)
	}
	return out
}

// TranslateDataPoints transforms datapoints to a format compatible with signalfx backend
// sfxDataPoints represents one metric converted to signalfx protobuf datapoints
func (mp *MetricTranslator) TranslateDataPoints(logger *zap.Logger, sfxDataPoints []*sfxpb.DataPoint) []*sfxpb.DataPoint {
	processedDataPoints := sfxDataPoints

	for _, tr := range mp.rules {
		switch tr.Action {
		case ActionRenameDimensionKeys:
			for _, dp := range processedDataPoints {
				if len(tr.MetricNames) > 0 && !tr.MetricNames[dp.Metric] {
					continue
				}
				for _, d := range dp.Dimensions {
					if newKey, ok := tr.Mapping[d.Key]; ok {
						d.Key = newKey
					}
				}
			}
		case ActionRenameMetrics:
			var additionalDimensions []*sfxpb.Dimension
			if tr.AddDimensions != nil {
				for k, v := range tr.AddDimensions {
					additionalDimensions = append(additionalDimensions, &sfxpb.Dimension{Key: k, Value: v})
				}
			}

			for _, dp := range processedDataPoints {
				if newKey, ok := tr.Mapping[dp.Metric]; ok {
					dp.Metric = newKey
					if tr.CopyDimensions != nil {
						for _, d := range dp.Dimensions {
							if k, ok := tr.CopyDimensions[d.Key]; ok {
								dp.Dimensions = append(dp.Dimensions, &sfxpb.Dimension{Key: k, Value: d.Value})

							}
						}
					}
					if len(additionalDimensions) > 0 {
						dp.Dimensions = append(dp.Dimensions, additionalDimensions...)
					}
				}
			}
		case ActionMultiplyInt:
			for _, dp := range processedDataPoints {
				if multiplier, ok := tr.ScaleFactorsInt[dp.Metric]; ok {
					v := dp.GetValue().IntValue
					if v != nil {
						*v *= multiplier
					}
				}
			}
		case ActionDivideInt:
			for _, dp := range processedDataPoints {
				if divisor, ok := tr.ScaleFactorsInt[dp.Metric]; ok {
					v := dp.GetValue().IntValue
					if v != nil {
						*v /= divisor
					}
				}
			}
		case ActionMultiplyFloat:
			for _, dp := range processedDataPoints {
				if multiplier, ok := tr.ScaleFactorsFloat[dp.Metric]; ok {
					v := dp.GetValue().DoubleValue
					if v != nil {
						*v *= multiplier
					}
				}
			}
		case ActionCopyMetrics:
			for _, dp := range processedDataPoints {
				if newMetric, ok := tr.Mapping[dp.Metric]; ok {
					newDataPoint := copyMetric(tr, dp, newMetric)
					if newDataPoint != nil {
						processedDataPoints = append(processedDataPoints, newDataPoint)
					}
				}
			}
		case ActionSplitMetric:
			for _, dp := range processedDataPoints {
				if tr.MetricName == dp.Metric {
					splitMetric(dp, tr.DimensionKey, tr.Mapping)
				}
			}
		case ActionConvertValues:
			for _, dp := range processedDataPoints {
				if newType, ok := tr.TypesMapping[dp.Metric]; ok {
					convertMetricValue(logger, dp, newType)
				}
			}
		case ActionCalculateNewMetric:
			pairs := calcNewMetricInputPairs(processedDataPoints, tr)
			for _, pair := range pairs {
				newPt := calculateNewMetric(logger, pair[0], pair[1], tr)
				if newPt == nil {
					continue
				}
				processedDataPoints = append(processedDataPoints, newPt)
			}

		case ActionAggregateMetric:
			// NOTE: Based on the usage of TranslateDataPoints we can assume that the datapoints batch []*sfxpb.DataPoint
			// represents only one metric and all the datapoints can be aggregated together.
			var dpsToAggregate []*sfxpb.DataPoint
			var otherDps []*sfxpb.DataPoint
			for i, dp := range processedDataPoints {
				if dp.Metric == tr.MetricName {
					if dpsToAggregate == nil {
						dpsToAggregate = make([]*sfxpb.DataPoint, 0, len(processedDataPoints)-i)
					}
					dpsToAggregate = append(dpsToAggregate, dp)
				} else {
					if otherDps == nil {
						otherDps = make([]*sfxpb.DataPoint, 0, len(processedDataPoints)-i)
					}
					// This slice can contain additional datapoints from a different metric
					// for example copied in a translation step before
					otherDps = append(otherDps, dp)
				}
			}
			aggregatedDps := aggregateDatapoints(dpsToAggregate, tr.WithoutDimensions, tr.AggregationMethod)
			processedDataPoints = otherDps
			processedDataPoints = append(processedDataPoints, aggregatedDps...)

		case ActionDropMetrics:
			resultSliceLen := 0
			for i, dp := range processedDataPoints {
				if match := tr.MetricNames[dp.Metric]; !match {
					if resultSliceLen < i {
						processedDataPoints[resultSliceLen] = dp
					}
					resultSliceLen++
				}
			}
			processedDataPoints = processedDataPoints[:resultSliceLen]

		case ActionDeltaMetric:
			processedDataPoints = mp.deltaTranslator.translate(processedDataPoints, tr)

		case ActionDropDimensions:
			for _, dp := range processedDataPoints {
				dropDimensions(dp, tr)
			}
		}
	}

	return processedDataPoints
}

func calcNewMetricInputPairs(processedDataPoints []*sfxpb.DataPoint, tr Rule) [][2]*sfxpb.DataPoint {
	var operand1Pts, operand2Pts []*sfxpb.DataPoint
	for _, dp := range processedDataPoints {
		if dp.Metric == tr.Operand1Metric {
			operand1Pts = append(operand1Pts, dp)
		} else if dp.Metric == tr.Operand2Metric {
			operand2Pts = append(operand2Pts, dp)
		}
	}
	var out [][2]*sfxpb.DataPoint
	for _, o1 := range operand1Pts {
		for _, o2 := range operand2Pts {
			if dimensionsEqual(o1.Dimensions, o2.Dimensions) {
				pair := [2]*sfxpb.DataPoint{o1, o2}
				out = append(out, pair)
			}
		}
	}
	return out
}

func dimensionsEqual(d1 []*sfxpb.Dimension, d2 []*sfxpb.Dimension) bool {
	if d1 == nil && d2 == nil {
		return true
	}
	if len(d1) != len(d2) {
		return false
	}
	// avoid allocating a map
	for _, dim1 := range d1 {
		matched := false
		for _, dim2 := range d2 {
			if dim1.Key == dim2.Key && dim1.Value == dim2.Value {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}

func calculateNewMetric(
	logger *zap.Logger,
	operand1 *sfxpb.DataPoint,
	operand2 *sfxpb.DataPoint,
	tr Rule,
) *sfxpb.DataPoint {
	v1 := ptToFloatVal(operand1)
	if v1 == nil {
		logger.Warn(
			"calculate_new_metric: operand1 has no numeric value",
			zap.String("tr.Operand1Metric", tr.Operand1Metric),
			zap.String("tr.MetricName", tr.MetricName),
		)
		return nil
	}

	v2 := ptToFloatVal(operand2)
	if v2 == nil {
		logger.Warn(
			"calculate_new_metric: operand2 has no numeric value",
			zap.String("tr.Operand2Metric", tr.Operand1Metric),
			zap.String("tr.MetricName", tr.MetricName),
		)
		return nil
	}

	if tr.Operator == MetricOperatorDivision && *v2 == 0 {
		// We can get here if, for example, in the denominator we get multiple
		// datapoints that have the same counter value, which will yield a delta of
		// zero.
		logger.Debug(
			"calculate_new_metric: attempt to divide by zero, skipping",
			zap.String("tr.Operand2Metric", tr.Operand2Metric),
			zap.String("tr.MetricName", tr.MetricName),
		)
		return nil
	}

	newPt := proto.Clone(operand1).(*sfxpb.DataPoint)
	newPt.Metric = tr.MetricName
	var newPtVal float64
	switch tr.Operator {
	// only supporting divide operator for now
	case MetricOperatorDivision:
		newPtVal = *v1 / *v2
	default:
		logger.Warn("calculate_new_metric: unsupported operator", zap.String("operator", string(tr.Operator)))
		return nil
	}
	newPt.Value = sfxpb.Datum{DoubleValue: &newPtVal}
	return newPt
}

func ptToFloatVal(pt *sfxpb.DataPoint) *float64 {
	if pt == nil {
		return nil
	}
	var f float64
	switch {
	case pt.Value.IntValue != nil:
		f = float64(*pt.Value.IntValue)
	case pt.Value.DoubleValue != nil:
		f = *pt.Value.DoubleValue
	default:
		return nil
	}
	return &f
}

func (mp *MetricTranslator) translateDimension(orig string) string {
	if translated, ok := mp.dimensionsMap[orig]; ok {
		return translated
	}
	return orig
}

// aggregateDatapoints aggregates datapoints assuming that they have
// the same Timestamp, MetricType, Metric and Source fields.
func aggregateDatapoints(
	dps []*sfxpb.DataPoint,
	withoutDimensions []string,
	aggregation AggregationMethod,
) []*sfxpb.DataPoint {
	if len(dps) == 0 {
		return nil
	}

	// group datapoints by dimension values
	dimValuesToDps := make(map[string][]*sfxpb.DataPoint, len(dps))
	for i, dp := range dps {
		aggregationKey := stringifyDimensions(dp.Dimensions, withoutDimensions)
		if _, ok := dimValuesToDps[aggregationKey]; !ok {
			// set slice capacity to the possible maximum = len(dps)-i to avoid reallocations
			dimValuesToDps[aggregationKey] = make([]*sfxpb.DataPoint, 0, len(dps)-i)
		}
		dimValuesToDps[aggregationKey] = append(dimValuesToDps[aggregationKey], dp)
	}

	// Get aggregated results
	result := make([]*sfxpb.DataPoint, 0, len(dimValuesToDps))
	for _, dps := range dimValuesToDps {
		dp := proto.Clone(dps[0]).(*sfxpb.DataPoint)
		dp.Dimensions = filterDimensions(dp.Dimensions, withoutDimensions)
		switch aggregation {
		case AggregationMethodCount:
			gauge := sfxpb.MetricType_GAUGE
			dp.MetricType = &gauge
			value := int64(len(dps))
			dp.Value = sfxpb.Datum{
				IntValue: &value,
			}
		case AggregationMethodSum:
			var intValue int64
			var floatValue float64
			value := sfxpb.Datum{}
			for _, dp := range dps {
				if dp.Value.IntValue != nil {
					intValue += *dp.Value.IntValue
					value.IntValue = &intValue
				}
				if dp.Value.DoubleValue != nil {
					floatValue += *dp.Value.DoubleValue
					value.DoubleValue = &floatValue
				}
			}
			dp.Value = value
		case AggregationMethodAvg:
			var mean float64
			for _, dp := range dps {
				if dp.Value.IntValue != nil {
					mean += float64(*dp.Value.IntValue)
				}
				if dp.Value.DoubleValue != nil {
					mean += *dp.Value.DoubleValue
				}
			}
			mean /= float64(len(dps))
			dp.Value = sfxpb.Datum{
				DoubleValue: &mean,
			}
		}
		result = append(result, dp)
	}

	return result
}

// stringifyDimensions turns the passed-in `dimensions` into a string while
// ignoring the passed-in `exclusions`. The result has the following form:
// dim1:val1//dim2:val2. Order is deterministic so this function can be used to
// generate map keys.
func stringifyDimensions(dimensions []*sfxpb.Dimension, exclusions []string) string {
	const aggregationKeyDelimiter = "//"
	var aggregationKeyParts = make([]string, 0, len(dimensions))
	for _, d := range dimensions {
		if !dimensionIn(d, exclusions) {
			aggregationKeyParts = append(aggregationKeyParts, fmt.Sprintf("%s:%s", d.Key, d.Value))
		}
	}
	sort.Strings(aggregationKeyParts)
	return strings.Join(aggregationKeyParts, aggregationKeyDelimiter)
}

// filterDimensions returns list of dimension excluding withoutDimensions
func filterDimensions(dimensions []*sfxpb.Dimension, withoutDimensions []string) []*sfxpb.Dimension {
	if len(dimensions) == 0 || len(dimensions)-len(withoutDimensions) <= 0 {
		return nil
	}
	result := make([]*sfxpb.Dimension, 0, len(dimensions)-len(withoutDimensions))
	for _, d := range dimensions {
		if !dimensionIn(d, withoutDimensions) {
			result = append(result, d)
		}
	}
	return result
}

// dimensionIn checks if the dimension found in the dimensionsKeysFilter
func dimensionIn(dimension *sfxpb.Dimension, dimensionsKeysFilter []string) bool {
	for _, dk := range dimensionsKeysFilter {
		if dimension.Key == dk {
			return true
		}
	}
	return false
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

func copyMetric(tr Rule, dp *sfxpb.DataPoint, newMetricName string) *sfxpb.DataPoint {
	if tr.DimensionKey != "" {
		var match bool
		for _, d := range dp.Dimensions {
			if d.Key == tr.DimensionKey {
				match = tr.DimensionValues[d.Value]
				break
			}
		}
		if !match {
			return nil
		}
	}

	newDataPoint := proto.Clone(dp).(*sfxpb.DataPoint)
	newDataPoint.Metric = newMetricName
	return newDataPoint
}

func dropDimensions(dp *sfxpb.DataPoint, rule Rule) {
	if rule.metricMatcher != nil && !rule.metricMatcher.Matches(dp.Metric) {
		return
	}
	processedDimensions := filterDimensionsByValues(dp.Dimensions, rule.DimensionPairs)
	if processedDimensions == nil {
		return
	}
	dp.Dimensions = processedDimensions
}

func filterDimensionsByValues(
	dimensions []*sfxpb.Dimension,
	dimensionPairs map[string]map[string]bool) []*sfxpb.Dimension {
	if len(dimensions) == 0 {
		return nil
	}
	result := make([]*sfxpb.Dimension, 0, len(dimensions))
	for _, d := range dimensions {
		// If a dimension key does not exist in dimensionMatcher,
		// it should not be dropped. If the key exists but there's
		// no matcher/empty matcher, drop the dimension for all values.
		if dimValMatcher, ok := dimensionPairs[d.Key]; ok {
			if len(dimValMatcher) > 0 && !dimValMatcher[d.Value] {
				result = append(result, d)
			}
		} else {
			result = append(result, d)
		}
	}

	return result
}
