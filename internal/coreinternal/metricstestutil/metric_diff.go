// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metricstestutil // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/metricstestutil"

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
	ExpectedValue any
	ActualValue   any
	Msg           string
}

func (mf MetricDiff) String() string {
	return fmt.Sprintf("{msg='%v' expected=[%v] actual=[%v]}\n", mf.Msg, mf.ExpectedValue, mf.ActualValue)
}

func DiffMetrics(diffs []*MetricDiff, expected, actual pmetric.Metrics) []*MetricDiff {
	return append(diffs, diffMetricData(expected, actual)...)
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

func diffMetricData(expected, actual pmetric.Metrics) []*MetricDiff {
	expectedRMSlice := expected.ResourceMetrics()
	actualRMSlice := actual.ResourceMetrics()
	return diffRMSlices(toSlice(expectedRMSlice), toSlice(actualRMSlice))
}

func toSlice(s pmetric.ResourceMetricsSlice) (out []pmetric.ResourceMetrics) {
	for i := 0; i < s.Len(); i++ {
		out = append(out, s.At(i))
	}
	return out
}

func DiffMetric(diffs []*MetricDiff, expected pmetric.Metric, actual pmetric.Metric) []*MetricDiff {
	var mismatch bool
	diffs, mismatch = diffMetricDescriptor(diffs, expected, actual)
	if mismatch {
		return diffs
	}
	//exhaustive:enforce
	switch actual.Type() {
	case pmetric.MetricTypeGauge:
		diffs = diffNumberPts(diffs, expected.Gauge().DataPoints(), actual.Gauge().DataPoints())
	case pmetric.MetricTypeSum:
		diffs = diff(diffs, expected.Sum().IsMonotonic(), actual.Sum().IsMonotonic(), "Sum IsMonotonic")
		diffs = diff(diffs, expected.Sum().AggregationTemporality(), actual.Sum().AggregationTemporality(), "Sum AggregationTemporality")
		diffs = diffNumberPts(diffs, expected.Sum().DataPoints(), actual.Sum().DataPoints())
	case pmetric.MetricTypeHistogram:
		diffs = diff(diffs, expected.Histogram().AggregationTemporality(), actual.Histogram().AggregationTemporality(), "Histogram AggregationTemporality")
		diffs = diffHistogramPts(diffs, expected.Histogram().DataPoints(), actual.Histogram().DataPoints())
	case pmetric.MetricTypeExponentialHistogram:
		diffs = diff(diffs, expected.ExponentialHistogram().AggregationTemporality(), actual.ExponentialHistogram().AggregationTemporality(), "ExponentialHistogram AggregationTemporality")
		diffs = diffExponentialHistogramPts(diffs, expected.ExponentialHistogram().DataPoints(), actual.ExponentialHistogram().DataPoints())
	case pmetric.MetricTypeSummary:
		// Note: Summary data points are not currently handled
		panic("unsupported test case for summary data")
	default:
		panic("unsupported test case")
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
	return diffValues(diffs, expected.Type(), actual.Type(), "Metric Type")
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
		exPt := expected.At(i)
		acPt := actual.At(i)

		diffs = diffMetricAttrs(diffs, exPt.Attributes(), acPt.Attributes())
		diffs, mismatch = diffValues(diffs, exPt.ValueType(), acPt.ValueType(), "NumberDataPoint Value Type")
		if mismatch {
			return diffs
		}
		switch exPt.ValueType() {
		case pmetric.NumberDataPointValueTypeInt:
			diffs = diff(diffs, exPt.IntValue(), acPt.IntValue(), "NumberDataPoint Value")
		case pmetric.NumberDataPointValueTypeDouble:
			diffs = diff(diffs, exPt.DoubleValue(), acPt.DoubleValue(), "NumberDataPoint Value")
		}
		diffExemplars(diffs, exPt.Exemplars(), acPt.Exemplars())
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
		diffs = diffHistogramPt(diffs, expected.At(i), actual.At(i))
	}
	return diffs
}

func diffHistogramPt(
	diffs []*MetricDiff,
	expected pmetric.HistogramDataPoint,
	actual pmetric.HistogramDataPoint,
) []*MetricDiff {
	diffs = diffMetricAttrs(diffs, expected.Attributes(), actual.Attributes())
	diffs = diff(diffs, expected.Count(), actual.Count(), "HistogramDataPoint Count")
	diffs = diff(diffs, expected.Sum(), actual.Sum(), "HistogramDataPoint Sum")
	// TODO: HasSum, Min, HasMin, Max, HasMax are not covered in tests.
	diffs = diff(diffs, expected.BucketCounts(), actual.BucketCounts(), "HistogramDataPoint BucketCounts")
	diffs = diff(diffs, expected.ExplicitBounds(), actual.ExplicitBounds(), "HistogramDataPoint ExplicitBounds")
	return diffExemplars(diffs, expected.Exemplars(), actual.Exemplars())
}

func diffExponentialHistogramPts(
	diffs []*MetricDiff,
	expected pmetric.ExponentialHistogramDataPointSlice,
	actual pmetric.ExponentialHistogramDataPointSlice,
) []*MetricDiff {
	var mismatch bool
	diffs, mismatch = diffValues(diffs, expected.Len(), actual.Len(), "ExponentialHistogramDataPointSlice len")
	if mismatch {
		return diffs
	}
	for i := 0; i < expected.Len(); i++ {
		diffs = diffExponentialHistogramPt(diffs, expected.At(i), actual.At(i))
	}
	return diffs
}

func diffExponentialHistogramPt(
	diffs []*MetricDiff,
	expected pmetric.ExponentialHistogramDataPoint,
	actual pmetric.ExponentialHistogramDataPoint,
) []*MetricDiff {
	diffs = diffMetricAttrs(diffs, expected.Attributes(), actual.Attributes())
	diffs = diff(diffs, expected.Count(), actual.Count(), "ExponentialHistogramDataPoint Count")
	diffs = diff(diffs, expected.HasSum(), actual.HasSum(), "ExponentialHistogramDataPoint HasSum")
	diffs = diff(diffs, expected.HasMin(), actual.HasMin(), "ExponentialHistogramDataPoint HasMin")
	diffs = diff(diffs, expected.HasMax(), actual.HasMax(), "ExponentialHistogramDataPoint HasMax")
	diffs = diff(diffs, expected.Sum(), actual.Sum(), "ExponentialHistogramDataPoint Sum")
	diffs = diff(diffs, expected.Min(), actual.Min(), "ExponentialHistogramDataPoint Min")
	diffs = diff(diffs, expected.Max(), actual.Max(), "ExponentialHistogramDataPoint Max")
	diffs = diff(diffs, expected.ZeroCount(), actual.ZeroCount(), "ExponentialHistogramDataPoint ZeroCount")
	diffs = diff(diffs, expected.Scale(), actual.Scale(), "ExponentialHistogramDataPoint Scale")

	diffs = diffExponentialHistogramPtBuckets(diffs, expected.Positive(), actual.Positive())
	diffs = diffExponentialHistogramPtBuckets(diffs, expected.Negative(), actual.Negative())

	return diffExemplars(diffs, expected.Exemplars(), actual.Exemplars())
}

func diffExponentialHistogramPtBuckets(
	diffs []*MetricDiff,
	expected pmetric.ExponentialHistogramDataPointBuckets,
	actual pmetric.ExponentialHistogramDataPointBuckets,
) []*MetricDiff {
	diffs = diff(diffs, expected.Offset(), actual.Offset(), "ExponentialHistogramDataPoint Buckets Offset")
	exC := expected.BucketCounts()
	acC := actual.BucketCounts()
	diffs, mod := diffValues(diffs, exC.Len(), acC.Len(), "ExponentialHistogramDataPoint Buckets Len")
	if mod {
		return diffs
	}
	for i := 0; i < exC.Len(); i++ {
		diffs = diff(diffs, exC.At(i), acC.At(i), fmt.Sprintf("ExponentialHistogramDataPoint Buckets Count[%d]", i))
	}
	return diffs
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
			diffs = diff(diffs, expected.At(i).IntValue(), actual.At(i).IntValue(), "Exemplar Value")
		case pmetric.ExemplarValueTypeDouble:
			diffs = diff(diffs, expected.At(i).DoubleValue(), actual.At(i).DoubleValue(), "Exemplar Value")
		}
	}
	return diffs
}

func diffResource(diffs []*MetricDiff, expected pcommon.Resource, actual pcommon.Resource) []*MetricDiff {
	return diffResourceAttrs(diffs, expected.Attributes(), actual.Attributes())
}

func diffResourceAttrs(diffs []*MetricDiff, expected pcommon.Map, actual pcommon.Map) []*MetricDiff {
	if !reflect.DeepEqual(expected, actual) {
		diffs = append(diffs, &MetricDiff{
			ExpectedValue: attrMapToString(expected),
			ActualValue:   attrMapToString(actual),
			Msg:           "Resource attributes",
		})
	}
	return diffs
}

func diffMetricAttrs(diffs []*MetricDiff, expected pcommon.Map, actual pcommon.Map) []*MetricDiff {
	if !reflect.DeepEqual(expected, actual) {
		diffs = append(diffs, &MetricDiff{
			ExpectedValue: attrMapToString(expected),
			ActualValue:   attrMapToString(actual),
			Msg:           "Metric attributes",
		})
	}
	return diffs
}

func diff(diffs []*MetricDiff, expected any, actual any, msg string) []*MetricDiff {
	out, _ := diffValues(diffs, expected, actual, msg)
	return out
}

func diffValues(
	diffs []*MetricDiff,
	expected any,
	actual any,
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
		out += "[" + k + "=" + v.Str() + "]"
		return true
	})
	return out
}
