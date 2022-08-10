// Copyright 2020 OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package metricstransformprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstransformprocessor"

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

// aggregateLabelsOp aggregates points that have the labels excluded in label_set
func (mtp *metricsTransformProcessor) aggregateLabelsOp(metric *metricspb.Metric, mtpOp internalOperation) {
	labelIdxs, labels := mtp.getLabelIdxs(metric, mtpOp.labelSetMap)
	groupedTimeseries := mtp.groupTimeseriesByLabelSet(metric.Timeseries, labelIdxs)
	aggregatedTimeseries := mtp.mergeTimeseries(groupedTimeseries, mtpOp.configOperation.AggregationType, metric.MetricDescriptor.Type)

	metric.MetricDescriptor.LabelKeys = labels

	mtp.sortTimeseries(aggregatedTimeseries)
	metric.Timeseries = aggregatedTimeseries
}

// groupTimeseries groups all the provided timeseries that will be aggregated together based on all the label values.
// Returns a map of grouped timeseries and the corresponding selected labels
func (mtp *metricsTransformProcessor) groupTimeseries(timeseries []*metricspb.TimeSeries, labelCount int) map[string]*timeseriesAndLabelValues {
	labelIdxs := make([]int, labelCount)
	for i := 0; i < labelCount; i++ {
		labelIdxs[i] = i
	}
	return mtp.groupTimeseriesByLabelSet(timeseries, labelIdxs)
}

// groupTimeseriesByLabelSet groups all the provided timeseries that will be aggregated together based on the selected label values as indicated by labelIdxs.
// Returns a map of grouped timeseries and the corresponding selected labels
func (mtp *metricsTransformProcessor) groupTimeseriesByLabelSet(timeseries []*metricspb.TimeSeries, labelIdxs []int) map[string]*timeseriesAndLabelValues {
	// key is a composite of the label values as a single string
	groupedTimeseries := make(map[string]*timeseriesAndLabelValues)
	for _, timeseries := range timeseries {
		key, newLabelValues := mtp.selectedLabelsAsKey(labelIdxs, timeseries)
		if timeseries.StartTimestamp != nil {
			key += strconv.FormatInt(timeseries.StartTimestamp.Seconds, 10)
		}

		timeseriesGroup, ok := groupedTimeseries[key]
		if ok {
			timeseriesGroup.timeseries = append(timeseriesGroup.timeseries, timeseries)
		} else {
			groupedTimeseries[key] = &timeseriesAndLabelValues{
				timeseries:  []*metricspb.TimeSeries{timeseries},
				labelValues: newLabelValues,
			}
		}
	}
	return groupedTimeseries
}

// selectedLabelsAsKey composes the key for the timeseries based on the selected labels' values indicated by labelIdxs
// Returns the key and a slice of the actual values used in this key
func (mtp *metricsTransformProcessor) selectedLabelsAsKey(labelIdxs []int, timeseries *metricspb.TimeSeries) (string, []*metricspb.LabelValue) {
	var key string
	newLabelValues := make([]*metricspb.LabelValue, len(labelIdxs))
	for i, vidx := range labelIdxs {
		key += fmt.Sprintf("%v-", timeseries.LabelValues[vidx].Value)
		newLabelValues[i] = timeseries.LabelValues[vidx]
	}
	return key, newLabelValues
}

type aggGroups struct {
	gauge        map[string]pmetric.NumberDataPointSlice
	sum          map[string]pmetric.NumberDataPointSlice
	histogram    map[string]pmetric.HistogramDataPointSlice
	expHistogram map[string]pmetric.ExponentialHistogramDataPointSlice
}

// aggregateLabelsOp aggregates points that have the labels excluded in label_set
func aggregateLabelsOp(metric pmetric.Metric, mtpOp internalOperation) {
	filterAttrs(metric, mtpOp.labelSetMap)
	newMetric := pmetric.NewMetric()
	copyMetricDetails(metric, newMetric)
	ag := groupDataPoints(metric, aggGroups{})
	mergeDataPoints(newMetric, mtpOp.configOperation.AggregationType, ag)
	newMetric.MoveTo(metric)
}

// groupMetrics groups all the provided timeseries that will be aggregated together based on all the label values.
// Returns a map of grouped timeseries and the corresponding selected labels
// canBeCombined must be callled before.
func groupMetrics(metrics pmetric.MetricSlice, aggType AggregationType, to pmetric.Metric) {
	var ag aggGroups
	for i := 0; i < metrics.Len(); i++ {
		ag = groupDataPoints(metrics.At(i), ag)
	}
	mergeDataPoints(to, aggType, ag)
}

func groupDataPoints(metric pmetric.Metric, ag aggGroups) aggGroups {
	switch metric.DataType() {
	case pmetric.MetricDataTypeGauge:
		if ag.gauge == nil {
			ag.gauge = map[string]pmetric.NumberDataPointSlice{}
		}
		groupNumberDataPoints(metric.Gauge().DataPoints(), ag.gauge)
	case pmetric.MetricDataTypeSum:
		if ag.sum == nil {
			ag.sum = map[string]pmetric.NumberDataPointSlice{}
		}
		groupNumberDataPoints(metric.Sum().DataPoints(), ag.sum)
	case pmetric.MetricDataTypeHistogram:
		if ag.histogram == nil {
			ag.histogram = map[string]pmetric.HistogramDataPointSlice{}
		}
		groupHistogramDataPoints(metric.Histogram().DataPoints(), ag.histogram)
	case pmetric.MetricDataTypeExponentialHistogram:
		if ag.expHistogram == nil {
			ag.expHistogram = map[string]pmetric.ExponentialHistogramDataPointSlice{}
		}
		groupExponentialHistogramDataPoints(metric.ExponentialHistogram().DataPoints(), ag.expHistogram)
	}
	return ag
}

func mergeDataPoints(to pmetric.Metric, aggType AggregationType, ag aggGroups) {
	switch to.DataType() {
	case pmetric.MetricDataTypeGauge:
		mergeNumberDataPoints(ag.gauge, aggType, to.Gauge().DataPoints())
	case pmetric.MetricDataTypeSum:
		mergeNumberDataPoints(ag.sum, aggType, to.Sum().DataPoints())
	case pmetric.MetricDataTypeHistogram:
		mergeHistogramDataPoints(ag.histogram, to.Histogram().DataPoints())
	case pmetric.MetricDataTypeExponentialHistogram:
		mergeExponentialHistogramDataPoints(ag.expHistogram, to.ExponentialHistogram().DataPoints())
	}
}

func groupNumberDataPoints(dps pmetric.NumberDataPointSlice, dpsByAttrsAndTs map[string]pmetric.NumberDataPointSlice) {
	for i := 0; i < dps.Len(); i++ {
		key := dataPointHashKey(dps.At(i).Attributes(), dps.At(i).StartTimestamp(), dps.At(i).Timestamp())
		if _, ok := dpsByAttrsAndTs[key]; !ok {
			dpsByAttrsAndTs[key] = pmetric.NewNumberDataPointSlice()
		}
		dps.At(i).MoveTo(dpsByAttrsAndTs[key].AppendEmpty())
	}
}

func groupHistogramDataPoints(dps pmetric.HistogramDataPointSlice,
	dpsByAttrsAndTs map[string]pmetric.HistogramDataPointSlice) {
	for i := 0; i < dps.Len(); i++ {
		dp := dps.At(i)
		keyHashParts := make([]interface{}, 0, dp.ExplicitBounds().Len()+3)
		for b := 0; b < dp.ExplicitBounds().Len(); b++ {
			keyHashParts = append(keyHashParts, dp.ExplicitBounds().At(b))
		}

		keyHashParts = append(keyHashParts, dp.HasMin(), dp.HasMax(), flagsValue(dp.FlagsStruct()))
		key := dataPointHashKey(dps.At(i).Attributes(), dp.StartTimestamp(), dp.Timestamp(), keyHashParts...)
		if _, ok := dpsByAttrsAndTs[key]; !ok {
			dpsByAttrsAndTs[key] = pmetric.NewHistogramDataPointSlice()
		}
		dp.MoveTo(dpsByAttrsAndTs[key].AppendEmpty())
	}
}

func groupExponentialHistogramDataPoints(dps pmetric.ExponentialHistogramDataPointSlice,
	dpsByAttrsAndTs map[string]pmetric.ExponentialHistogramDataPointSlice) {
	for i := 0; i < dps.Len(); i++ {
		dp := dps.At(i)
		keyHashParts := make([]interface{}, 0, 4)
		keyHashParts = append(keyHashParts, dp.Scale(), dp.HasMin(), dp.HasMax(), flagsValue(dp.FlagsStruct()), dp.Negative().Offset(),
			dp.Positive().Offset())
		key := dataPointHashKey(dps.At(i).Attributes(), dp.StartTimestamp(), dp.Timestamp(), keyHashParts...)
		if _, ok := dpsByAttrsAndTs[key]; !ok {
			dpsByAttrsAndTs[key] = pmetric.NewExponentialHistogramDataPointSlice()
		}
		dp.MoveTo(dpsByAttrsAndTs[key].AppendEmpty())
	}
}

func flagsValue(flags pmetric.MetricDataPointFlagsStruct) uint32 {
	if flags.NoRecordedValue() {
		return uint32(1)
	}
	return uint32(0)
}

func filterAttrs(metric pmetric.Metric, filterAttrKeys map[string]bool) {
	if filterAttrKeys == nil {
		return
	}
	rangeDataPointAttributes(metric, func(attrs pcommon.Map) bool {
		attrs.RemoveIf(func(k string, v pcommon.Value) bool {
			return !filterAttrKeys[k]
		})
		return true
	})
}

func dataPointHashKey(atts pcommon.Map, start pcommon.Timestamp, ts pcommon.Timestamp, other ...interface{}) string {
	hashParts := []interface{}{atts.AsRaw(), start.String(), ts.String()}
	jsonStr, _ := json.Marshal(append(hashParts, other...))
	return string(jsonStr)
}

func mergeNumberDataPoints(dpsMap map[string]pmetric.NumberDataPointSlice, agg AggregationType, to pmetric.NumberDataPointSlice) {
	for _, dps := range dpsMap {
		dp := to.AppendEmpty()
		dps.At(0).MoveTo(dp)
		switch dp.ValueType() {
		case pmetric.NumberDataPointValueTypeDouble:
			for i := 1; i < dps.Len(); i++ {
				switch agg {
				case Sum, Mean:
					dp.SetDoubleVal(dp.DoubleVal() + doubleVal(dps.At(i)))
				case Max:
					dp.SetDoubleVal(math.Max(dp.DoubleVal(), doubleVal(dps.At(i))))
				case Min:
					dp.SetDoubleVal(math.Min(dp.DoubleVal(), doubleVal(dps.At(i))))
				}
			}
			if agg == Mean {
				dp.SetDoubleVal(dp.DoubleVal() / float64(dps.Len()))
			}
		case pmetric.NumberDataPointValueTypeInt:
			for i := 1; i < dps.Len(); i++ {
				switch agg {
				case Sum, Mean:
					dp.SetIntVal(dp.IntVal() + dps.At(i).IntVal())
				case Max:
					if dp.IntVal() < intVal(dps.At(i)) {
						dp.SetIntVal(intVal(dps.At(i)))
					}
				case Min:
					if dp.IntVal() > intVal(dps.At(i)) {
						dp.SetIntVal(intVal(dps.At(i)))
					}
				}
			}
			if agg == Mean {
				dp.SetIntVal(dp.IntVal() / int64(dps.Len()))
			}
		}
	}
}

func doubleVal(dp pmetric.NumberDataPoint) float64 {
	switch dp.ValueType() {
	case pmetric.NumberDataPointValueTypeDouble:
		return dp.DoubleVal()
	case pmetric.NumberDataPointValueTypeInt:
		return float64(dp.IntVal())
	}
	return 0
}

func intVal(dp pmetric.NumberDataPoint) int64 {
	switch dp.ValueType() {
	case pmetric.NumberDataPointValueTypeDouble:
		return int64(dp.DoubleVal())
	case pmetric.NumberDataPointValueTypeInt:
		return dp.IntVal()
	}
	return 0
}

func mergeHistogramDataPoints(dpsMap map[string]pmetric.HistogramDataPointSlice, to pmetric.HistogramDataPointSlice) {
	for _, dps := range dpsMap {
		dp := to.AppendEmpty()
		dps.At(0).MoveTo(dp)
		counts := dp.BucketCounts().AsRaw()
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
			dps.At(i).Exemplars().MoveAndAppendTo(dp.Exemplars())
			for b := 0; b < dps.At(i).BucketCounts().Len(); b++ {
				counts[b] += dps.At(i).BucketCounts().At(b)
			}
			dps.At(i).Exemplars().MoveAndAppendTo(dp.Exemplars())
		}
		dp.SetBucketCounts(pcommon.NewImmutableUInt64Slice(counts))
	}
}

func mergeExponentialHistogramDataPoints(dpsMap map[string]pmetric.ExponentialHistogramDataPointSlice,
	to pmetric.ExponentialHistogramDataPointSlice) {
	for _, dps := range dpsMap {
		dp := to.AppendEmpty()
		dps.At(0).MoveTo(dp)
		negatives := dp.Negative().BucketCounts().AsRaw()
		positives := dp.Positive().BucketCounts().AsRaw()
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
			for b := 0; b < dps.At(i).Negative().BucketCounts().Len(); b++ {
				negatives[b] += dps.At(i).Negative().BucketCounts().At(b)
			}
			for b := 0; b < dps.At(i).Positive().BucketCounts().Len(); b++ {
				positives[b] += dps.At(i).Positive().BucketCounts().At(b)
			}
			dps.At(i).Exemplars().MoveAndAppendTo(dp.Exemplars())
		}
		dp.Negative().SetBucketCounts(pcommon.NewImmutableUInt64Slice(negatives))
		dp.Positive().SetBucketCounts(pcommon.NewImmutableUInt64Slice(positives))
	}
}
