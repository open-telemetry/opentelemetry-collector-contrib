// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package metrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/testbed/correctnesstests/metrics"

import (
	"fmt"
	"reflect"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

// MetricDiff is intended to support producing human-readable diffs between two MetricData structs during
// testing. Two MetricDatas, when compared, could produce a list of MetricDiffs containing all of their
// differences, which could be used to correct the differences between the expected and actual values.
type MetricDiff struct {
	ExpectedValue interface{}
	ActualValue   interface{}
	Msg           string
}

func (mf MetricDiff) String() string {
	return fmt.Sprintf("{msg='%v' expected=[%v] actual=[%v]}\n", mf.Msg, mf.ExpectedValue, mf.ActualValue)
}

func diffRMSlices(sent []pmetric.ResourceMetrics, recd []pmetric.ResourceMetrics) []*MetricDiff {
	var diffs []*MetricDiff
	if len(sent) != len(recd) {
		return []*MetricDiff{{
			ExpectedValue: len(sent),
			ActualValue:   len(recd),
			Msg:           "Sent vs received ResourceMetrics not equal length",
		}}
	}
	for i := 0; i < len(sent); i++ {
		sentRM := sent[i]
		recdRM := recd[i]
		diffs = diffRMs(diffs, sentRM, recdRM)
	}
	return diffs
}

func diffRMs(diffs []*MetricDiff, expected pmetric.ResourceMetrics, actual pmetric.ResourceMetrics) []*MetricDiff {
	diffs = diffResource(diffs, expected.Resource(), actual.Resource())
	diffs = diffILMSlice(
		diffs,
		expected.ScopeMetrics(),
		actual.ScopeMetrics(),
	)
	return diffs
}

func diffILMSlice(
	diffs []*MetricDiff,
	expected pmetric.ScopeMetricsSlice,
	actual pmetric.ScopeMetricsSlice,
) []*MetricDiff {
	var mismatch bool
	diffs, mismatch = diffValues(diffs, actual.Len(), expected.Len(), "ScopeMetricsSlice len")
	if mismatch {
		return diffs
	}
	for i := 0; i < expected.Len(); i++ {
		diffs = diffILM(diffs, expected.At(i), actual.At(i))
	}
	return diffs
}

func diffILM(
	diffs []*MetricDiff,
	expected pmetric.ScopeMetrics,
	actual pmetric.ScopeMetrics,
) []*MetricDiff {
	return diffMetrics(diffs, expected.Metrics(), actual.Metrics())
}

func diffMetrics(diffs []*MetricDiff, expected pmetric.MetricSlice, actual pmetric.MetricSlice) []*MetricDiff {
	var mismatch bool
	diffs, mismatch = diffValues(diffs, actual.Len(), expected.Len(), "MetricSlice len")
	if mismatch {
		return diffs
	}
	for i := 0; i < expected.Len(); i++ {
		diffs = DiffMetric(diffs, expected.At(i), actual.At(i))
	}
	return diffs
}

func DiffMetric(diffs []*MetricDiff, expected pmetric.Metric, actual pmetric.Metric) []*MetricDiff {
	var mismatch bool
	diffs, mismatch = diffMetricDescriptor(diffs, expected, actual)
	if mismatch {
		return diffs
	}
	switch actual.DataType() {
	case pmetric.MetricDataTypeGauge:
		diffs = diffNumberPts(diffs, expected.Gauge().DataPoints(), actual.Gauge().DataPoints())
	case pmetric.MetricDataTypeSum:
		diffs = diff(diffs, expected.Sum().IsMonotonic(), actual.Sum().IsMonotonic(), "Sum IsMonotonic")
		diffs = diff(diffs, expected.Sum().AggregationTemporality(), actual.Sum().AggregationTemporality(), "Sum AggregationTemporality")
		diffs = diffNumberPts(diffs, expected.Sum().DataPoints(), actual.Sum().DataPoints())
	case pmetric.MetricDataTypeHistogram:
		diffs = diff(diffs, expected.Histogram().AggregationTemporality(), actual.Histogram().AggregationTemporality(), "Histogram AggregationTemporality")
		diffs = diffHistogramPts(diffs, expected.Histogram().DataPoints(), actual.Histogram().DataPoints())
	}
	return diffs
}

func diffMetricDescriptor(
	diffs []*MetricDiff,
	expected pmetric.Metric,
	actual pmetric.Metric,
) ([]*MetricDiff, bool) {
	diffs = diff(diffs, expected.Name(), actual.Name(), "Metric Name")
	diffs = diff(diffs, expected.Description(), actual.Description(), "Metric Description")
	diffs = diff(diffs, expected.Unit(), actual.Unit(), "Metric Unit")
	return diffValues(diffs, expected.DataType(), actual.DataType(), "Metric Type")
}

func diffNumberPts(
	diffs []*MetricDiff,
	expected pmetric.NumberDataPointSlice,
	actual pmetric.NumberDataPointSlice,
) []*MetricDiff {
	var mismatch bool
	diffs, mismatch = diffValues(diffs, expected.Len(), actual.Len(), "NumberDataPointSlice len")
	if mismatch {
		return diffs
	}
	for i := 0; i < expected.Len(); i++ {
		diffs, mismatch = diffValues(diffs, expected.At(i).ValueType(), actual.At(i).ValueType(), "NumberDataPoint Value Type")
		if mismatch {
			return diffs
		}
		switch expected.At(i).ValueType() {
		case pmetric.NumberDataPointValueTypeInt:
			diffs = diff(diffs, expected.At(i).IntVal(), actual.At(i).IntVal(), "NumberDataPoint Value")
		case pmetric.NumberDataPointValueTypeDouble:
			diffs = diff(diffs, expected.At(i).DoubleVal(), actual.At(i).DoubleVal(), "NumberDataPoint Value")
		}
		diffExemplars(diffs, expected.At(i).Exemplars(), actual.At(i).Exemplars())
	}
	return diffs
}

func diffHistogramPts(
	diffs []*MetricDiff,
	expected pmetric.HistogramDataPointSlice,
	actual pmetric.HistogramDataPointSlice,
) []*MetricDiff {
	var mismatch bool
	diffs, mismatch = diffValues(diffs, expected.Len(), actual.Len(), "HistogramDataPointSlice len")
	if mismatch {
		return diffs
	}
	for i := 0; i < expected.Len(); i++ {
		diffs = diffDoubleHistogramPt(diffs, expected.At(i), actual.At(i))
	}
	return diffs
}

func diffDoubleHistogramPt(
	diffs []*MetricDiff,
	expected pmetric.HistogramDataPoint,
	actual pmetric.HistogramDataPoint,
) []*MetricDiff {
	diffs = diff(diffs, expected.Count(), actual.Count(), "HistogramDataPoint Count")
	diffs = diff(diffs, expected.Sum(), actual.Sum(), "HistogramDataPoint Sum")
	diffs = diff(diffs, expected.BucketCounts(), actual.BucketCounts(), "HistogramDataPoint BucketCounts")
	diffs = diff(diffs, expected.ExplicitBounds(), actual.ExplicitBounds(), "HistogramDataPoint ExplicitBounds")
	// todo LabelsMap()
	return diffExemplars(diffs, expected.Exemplars(), actual.Exemplars())
}

func diffExemplars(
	diffs []*MetricDiff,
	expected pmetric.ExemplarSlice,
	actual pmetric.ExemplarSlice,
) []*MetricDiff {
	var mismatch bool
	diffs, mismatch = diffValues(diffs, expected.Len(), actual.Len(), "ExemplarSlice len")
	if mismatch {
		return diffs
	}
	for i := 0; i < expected.Len(); i++ {
		diffs = diff(diffs, expected.At(i).ValueType(), actual.At(i).ValueType(), "Exemplar Value Type")
		switch expected.At(i).ValueType() {
		case pmetric.ExemplarValueTypeInt:
			diffs = diff(diffs, expected.At(i).IntVal(), actual.At(i).IntVal(), "Exemplar Value")
		case pmetric.ExemplarValueTypeDouble:
			diffs = diff(diffs, expected.At(i).DoubleVal(), actual.At(i).DoubleVal(), "Exemplar Value")
		}
	}
	return diffs
}

func diffResource(diffs []*MetricDiff, expected pcommon.Resource, actual pcommon.Resource) []*MetricDiff {
	return diffAttrs(diffs, expected.Attributes(), actual.Attributes())
}

func diffAttrs(diffs []*MetricDiff, expected pcommon.Map, actual pcommon.Map) []*MetricDiff {
	if !reflect.DeepEqual(expected, actual) {
		diffs = append(diffs, &MetricDiff{
			ExpectedValue: attrMapToString(expected),
			ActualValue:   attrMapToString(actual),
			Msg:           "Resource attributes",
		})
	}
	return diffs
}

func diff(diffs []*MetricDiff, expected interface{}, actual interface{}, msg string) []*MetricDiff {
	out, _ := diffValues(diffs, expected, actual, msg)
	return out
}

func diffValues(
	diffs []*MetricDiff,
	expected interface{},
	actual interface{},
	msg string,
) ([]*MetricDiff, bool) {
	if !reflect.DeepEqual(expected, actual) {
		return append(diffs, &MetricDiff{
			Msg:           msg,
			ExpectedValue: expected,
			ActualValue:   actual,
		}), true
	}
	return diffs, false
}

func attrMapToString(m pcommon.Map) string {
	out := ""
	m.Range(func(k string, v pcommon.Value) bool {
		out += "[" + k + "=" + v.StringVal() + "]"
		return true
	})
	return out
}
