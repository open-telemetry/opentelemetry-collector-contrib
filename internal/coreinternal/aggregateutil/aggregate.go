// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package aggregateutil // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/aggregateutil"

import (
	"encoding/json"
	"math"
	"sort"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func CopyMetricDetails(from, to pmetric.Metric) {
	to.SetName(from.Name())
	to.SetUnit(from.Unit())
	to.SetDescription(from.Description())
	//exhaustive:enforce
	switch from.Type() {
	case pmetric.MetricTypeGauge:
		to.SetEmptyGauge()
	case pmetric.MetricTypeSum:
		to.SetEmptySum().SetAggregationTemporality(from.Sum().AggregationTemporality())
		to.Sum().SetIsMonotonic(from.Sum().IsMonotonic())
	case pmetric.MetricTypeHistogram:
		to.SetEmptyHistogram().SetAggregationTemporality(from.Histogram().AggregationTemporality())
	case pmetric.MetricTypeExponentialHistogram:
		to.SetEmptyExponentialHistogram().SetAggregationTemporality(from.ExponentialHistogram().AggregationTemporality())
	case pmetric.MetricTypeSummary:
		to.SetEmptySummary()
	}
}

func FilterAttrs(metric pmetric.Metric, filterAttrKeys []string) {
	// filterAttrKeys being nil means the filter is to be skipped.
	if filterAttrKeys == nil {
		return
	}
	// filterAttrKeys being empty means it is explicitly expected to filter
	// against an empty label set, which is functionally the same as removing
	// all attributes.
	if len(filterAttrKeys) == 0 {
		RangeDataPointAttributes(metric, func(attrs pcommon.Map) bool {
			attrs.Clear()
			return true
		})
	}
	// filterAttrKeys having provided attributes means the filter continues
	// as normal.
	RangeDataPointAttributes(metric, func(attrs pcommon.Map) bool {
		attrs.RemoveIf(func(k string, _ pcommon.Value) bool {
			return isNotPresent(k, filterAttrKeys)
		})
		return true
	})
}

func GroupDataPoints(metric pmetric.Metric, ag *AggGroups) {
	switch metric.Type() {
	case pmetric.MetricTypeGauge:
		if ag.gauge == nil {
			ag.gauge = map[string]pmetric.NumberDataPointSlice{}
		}
		groupNumberDataPoints(metric.Gauge().DataPoints(), false, ag.gauge)
	case pmetric.MetricTypeSum:
		if ag.sum == nil {
			ag.sum = map[string]pmetric.NumberDataPointSlice{}
		}
		groupByStartTime := metric.Sum().AggregationTemporality() == pmetric.AggregationTemporalityDelta
		groupNumberDataPoints(metric.Sum().DataPoints(), groupByStartTime, ag.sum)
	case pmetric.MetricTypeHistogram:
		if ag.histogram == nil {
			ag.histogram = map[string]pmetric.HistogramDataPointSlice{}
		}
		groupByStartTime := metric.Histogram().AggregationTemporality() == pmetric.AggregationTemporalityDelta
		groupHistogramDataPoints(metric.Histogram().DataPoints(), groupByStartTime, ag.histogram)
	case pmetric.MetricTypeExponentialHistogram:
		if ag.expHistogram == nil {
			ag.expHistogram = map[string]pmetric.ExponentialHistogramDataPointSlice{}
		}
		groupByStartTime := metric.ExponentialHistogram().AggregationTemporality() == pmetric.AggregationTemporalityDelta
		groupExponentialHistogramDataPoints(metric.ExponentialHistogram().DataPoints(), groupByStartTime, ag.expHistogram)
	}
}

func MergeDataPoints(to pmetric.Metric, aggType AggregationType, ag AggGroups) {
	switch to.Type() {
	case pmetric.MetricTypeGauge:
		mergeNumberDataPoints(ag.gauge, aggType, to.Gauge().DataPoints())
	case pmetric.MetricTypeSum:
		mergeNumberDataPoints(ag.sum, aggType, to.Sum().DataPoints())
	case pmetric.MetricTypeHistogram:
		mergeHistogramDataPoints(ag.histogram, to.Histogram().DataPoints())
	case pmetric.MetricTypeExponentialHistogram:
		mergeExponentialHistogramDataPoints(ag.expHistogram, to.ExponentialHistogram().DataPoints())
	}
}

// RangeDataPointAttributes calls f sequentially on attributes of every metric data point.
// The iteration terminates if f returns false.
func RangeDataPointAttributes(metric pmetric.Metric, f func(pcommon.Map) bool) {
	//exhaustive:enforce
	switch metric.Type() {
	case pmetric.MetricTypeGauge:
		for i := 0; i < metric.Gauge().DataPoints().Len(); i++ {
			dp := metric.Gauge().DataPoints().At(i)
			if !f(dp.Attributes()) {
				return
			}
		}
	case pmetric.MetricTypeSum:
		for i := 0; i < metric.Sum().DataPoints().Len(); i++ {
			dp := metric.Sum().DataPoints().At(i)
			if !f(dp.Attributes()) {
				return
			}
		}
	case pmetric.MetricTypeHistogram:
		for i := 0; i < metric.Histogram().DataPoints().Len(); i++ {
			dp := metric.Histogram().DataPoints().At(i)
			if !f(dp.Attributes()) {
				return
			}
		}
	case pmetric.MetricTypeExponentialHistogram:
		for i := 0; i < metric.ExponentialHistogram().DataPoints().Len(); i++ {
			dp := metric.ExponentialHistogram().DataPoints().At(i)
			if !f(dp.Attributes()) {
				return
			}
		}
	case pmetric.MetricTypeSummary:
		for i := 0; i < metric.Summary().DataPoints().Len(); i++ {
			dp := metric.Summary().DataPoints().At(i)
			if !f(dp.Attributes()) {
				return
			}
		}
	}
}

func isNotPresent(target string, arr []string) bool {
	for _, item := range arr {
		if item == target {
			return false
		}
	}
	return true
}

func mergeNumberDataPoints(dpsMap map[string]pmetric.NumberDataPointSlice, agg AggregationType, to pmetric.NumberDataPointSlice) {
	for _, dps := range dpsMap {
		dp := to.AppendEmpty()
		dps.At(0).MoveTo(dp)
		switch dp.ValueType() {
		case pmetric.NumberDataPointValueTypeDouble:
			medianNumbers := []float64{dp.DoubleValue()}
			for i := 1; i < dps.Len(); i++ {
				switch agg {
				case Sum, Mean:
					dp.SetDoubleValue(dp.DoubleValue() + doubleVal(dps.At(i)))
				case Max:
					dp.SetDoubleValue(math.Max(dp.DoubleValue(), doubleVal(dps.At(i))))
				case Min:
					dp.SetDoubleValue(math.Min(dp.DoubleValue(), doubleVal(dps.At(i))))
				case Median:
					medianNumbers = append(medianNumbers, doubleVal(dps.At(i)))
				case Count:
					dp.SetDoubleValue(float64(dps.Len()))
				}
				if dps.At(i).StartTimestamp() < dp.StartTimestamp() {
					dp.SetStartTimestamp(dps.At(i).StartTimestamp())
				}
			}
			if agg == Mean {
				dp.SetDoubleValue(dp.DoubleValue() / float64(dps.Len()))
			}
			if agg == Median {
				if len(medianNumbers) == 1 {
					dp.SetDoubleValue(medianNumbers[0])
				} else {
					sort.Float64s(medianNumbers)
					mNumber := len(medianNumbers) / 2
					if math.Mod(float64(len(medianNumbers)), 2) != 0 {
						dp.SetDoubleValue(medianNumbers[mNumber])
					} else {
						dp.SetDoubleValue((medianNumbers[mNumber-1] + medianNumbers[mNumber]) / 2)
					}
				}
			}
		case pmetric.NumberDataPointValueTypeInt:
			medianNumbers := []int64{dp.IntValue()}
			for i := 1; i < dps.Len(); i++ {
				switch agg {
				case Sum, Mean:
					dp.SetIntValue(dp.IntValue() + dps.At(i).IntValue())
				case Max:
					if dp.IntValue() < intVal(dps.At(i)) {
						dp.SetIntValue(intVal(dps.At(i)))
					}
				case Min:
					if dp.IntValue() > intVal(dps.At(i)) {
						dp.SetIntValue(intVal(dps.At(i)))
					}
				case Median:
					medianNumbers = append(medianNumbers, intVal(dps.At(i)))
				case Count:
					dp.SetIntValue(int64(dps.Len()))
				}
				if dps.At(i).StartTimestamp() < dp.StartTimestamp() {
					dp.SetStartTimestamp(dps.At(i).StartTimestamp())
				}
			}
			if agg == Median {
				if len(medianNumbers) == 1 {
					dp.SetIntValue(medianNumbers[0])
				} else {
					sort.Slice(medianNumbers, func(i, j int) bool {
						return medianNumbers[i] < medianNumbers[j]
					})
					mNumber := len(medianNumbers) / 2
					if math.Mod(float64(len(medianNumbers)), 2) != 0 {
						dp.SetIntValue(medianNumbers[mNumber])
					} else {
						dp.SetIntValue((medianNumbers[mNumber-1] + medianNumbers[mNumber]) / 2)
					}
				}
			}
			if agg == Mean {
				dp.SetIntValue(dp.IntValue() / int64(dps.Len()))
			}
		}
	}
}

func doubleVal(dp pmetric.NumberDataPoint) float64 {
	switch dp.ValueType() {
	case pmetric.NumberDataPointValueTypeDouble:
		return dp.DoubleValue()
	case pmetric.NumberDataPointValueTypeInt:
		return float64(dp.IntValue())
	}
	return 0
}

func intVal(dp pmetric.NumberDataPoint) int64 {
	switch dp.ValueType() {
	case pmetric.NumberDataPointValueTypeDouble:
		return int64(dp.DoubleValue())
	case pmetric.NumberDataPointValueTypeInt:
		return dp.IntValue()
	}
	return 0
}

func mergeHistogramDataPoints(dpsMap map[string]pmetric.HistogramDataPointSlice, to pmetric.HistogramDataPointSlice) {
	for _, dps := range dpsMap {
		dp := to.AppendEmpty()
		dps.At(0).MoveTo(dp)
		counts := dp.BucketCounts()
		for i := 1; i < dps.Len(); i++ {
			if dps.At(i).Count() == 0 {
				continue
			}
			dp.SetCount(dp.Count() + dps.At(i).Count())
			dp.SetSum(dp.Sum() + dps.At(i).Sum())
			if dp.HasMin() && dp.Min() > dps.At(i).Min() {
				dp.SetMin(dps.At(i).Min())
			}
			if dp.HasMax() && dp.Max() < dps.At(i).Max() {
				dp.SetMax(dps.At(i).Max())
			}
			for b := 0; b < dps.At(i).BucketCounts().Len(); b++ {
				counts.SetAt(b, counts.At(b)+dps.At(i).BucketCounts().At(b))
			}
			dps.At(i).Exemplars().MoveAndAppendTo(dp.Exemplars())
			if dps.At(i).StartTimestamp() < dp.StartTimestamp() {
				dp.SetStartTimestamp(dps.At(i).StartTimestamp())
			}
		}
	}
}

func mergeExponentialHistogramDataPoints(dpsMap map[string]pmetric.ExponentialHistogramDataPointSlice,
	to pmetric.ExponentialHistogramDataPointSlice,
) {
	for _, dps := range dpsMap {
		dp := to.AppendEmpty()
		dps.At(0).MoveTo(dp)
		negatives := dp.Negative().BucketCounts()
		positives := dp.Positive().BucketCounts()
		for i := 1; i < dps.Len(); i++ {
			if dps.At(i).Count() == 0 {
				continue
			}
			dp.SetCount(dp.Count() + dps.At(i).Count())
			dp.SetSum(dp.Sum() + dps.At(i).Sum())
			dp.SetZeroCount(dp.ZeroCount() + dps.At(i).ZeroCount())
			if dp.HasMin() && dp.Min() > dps.At(i).Min() {
				dp.SetMin(dps.At(i).Min())
			}
			if dp.HasMax() && dp.Max() < dps.At(i).Max() {
				dp.SetMax(dps.At(i).Max())
			}
			// Merge bucket counts.
			// Note that groupExponentialHistogramDataPoints() has already ensured that we only try
			// to merge exponential histograms with matching Scale and Positive/Negative Offsets,
			// so the corresponding array items in BucketCounts have the same bucket boundaries.
			// However, the number of buckets may differ depending on what values have been observed.
			for b := 0; b < dps.At(i).Negative().BucketCounts().Len(); b++ {
				if b < negatives.Len() {
					negatives.SetAt(b, negatives.At(b)+dps.At(i).Negative().BucketCounts().At(b))
				} else {
					negatives.Append(dps.At(i).Negative().BucketCounts().At(b))
				}
			}
			for b := 0; b < dps.At(i).Positive().BucketCounts().Len(); b++ {
				if b < positives.Len() {
					positives.SetAt(b, positives.At(b)+dps.At(i).Positive().BucketCounts().At(b))
				} else {
					positives.Append(dps.At(i).Positive().BucketCounts().At(b))
				}
			}
			dps.At(i).Exemplars().MoveAndAppendTo(dp.Exemplars())
			if dps.At(i).StartTimestamp() < dp.StartTimestamp() {
				dp.SetStartTimestamp(dps.At(i).StartTimestamp())
			}
		}
	}
}

func groupNumberDataPoints(dps pmetric.NumberDataPointSlice, useStartTime bool,
	dpsByAttrsAndTs map[string]pmetric.NumberDataPointSlice,
) {
	var keyHashParts []any
	for i := 0; i < dps.Len(); i++ {
		if useStartTime {
			keyHashParts = []any{dps.At(i).StartTimestamp().String()}
		}
		key := dataPointHashKey(dps.At(i).Attributes(), dps.At(i).Timestamp(), keyHashParts...)
		if _, ok := dpsByAttrsAndTs[key]; !ok {
			dpsByAttrsAndTs[key] = pmetric.NewNumberDataPointSlice()
		}
		dps.At(i).MoveTo(dpsByAttrsAndTs[key].AppendEmpty())
	}
}

func groupHistogramDataPoints(dps pmetric.HistogramDataPointSlice, useStartTime bool,
	dpsByAttrsAndTs map[string]pmetric.HistogramDataPointSlice,
) {
	for i := 0; i < dps.Len(); i++ {
		dp := dps.At(i)
		keyHashParts := make([]any, 0, dp.ExplicitBounds().Len()+4)
		for b := 0; b < dp.ExplicitBounds().Len(); b++ {
			keyHashParts = append(keyHashParts, dp.ExplicitBounds().At(b))
		}
		if useStartTime {
			keyHashParts = append(keyHashParts, dp.StartTimestamp().String())
		}

		keyHashParts = append(keyHashParts, dp.HasMin(), dp.HasMax(), uint32(dp.Flags()))
		key := dataPointHashKey(dps.At(i).Attributes(), dp.Timestamp(), keyHashParts...)
		if _, ok := dpsByAttrsAndTs[key]; !ok {
			dpsByAttrsAndTs[key] = pmetric.NewHistogramDataPointSlice()
		}
		dp.MoveTo(dpsByAttrsAndTs[key].AppendEmpty())
	}
}

func groupExponentialHistogramDataPoints(dps pmetric.ExponentialHistogramDataPointSlice, useStartTime bool,
	dpsByAttrsAndTs map[string]pmetric.ExponentialHistogramDataPointSlice,
) {
	for i := 0; i < dps.Len(); i++ {
		dp := dps.At(i)
		keyHashParts := make([]any, 0, 5)
		keyHashParts = append(keyHashParts, dp.Scale(), dp.HasMin(), dp.HasMax(), uint32(dp.Flags()), dp.Negative().Offset(),
			dp.Positive().Offset())
		if useStartTime {
			keyHashParts = append(keyHashParts, dp.StartTimestamp().String())
		}
		key := dataPointHashKey(dps.At(i).Attributes(), dp.Timestamp(), keyHashParts...)
		if _, ok := dpsByAttrsAndTs[key]; !ok {
			dpsByAttrsAndTs[key] = pmetric.NewExponentialHistogramDataPointSlice()
		}
		dp.MoveTo(dpsByAttrsAndTs[key].AppendEmpty())
	}
}

func dataPointHashKey(atts pcommon.Map, ts pcommon.Timestamp, other ...any) string {
	hashParts := []any{atts.AsRaw(), ts.String()}
	jsonStr, _ := json.Marshal(append(hashParts, other...))
	return string(jsonStr)
}
