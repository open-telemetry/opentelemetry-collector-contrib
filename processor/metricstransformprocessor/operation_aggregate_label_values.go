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

package metricstransformprocessor

import (
	"fmt"
	"strconv"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
)

// aggregateLabelValuesOp aggregates points that have the label values specified in aggregated_values
func (mtp *metricsTransformProcessor) aggregateLabelValuesOp(metric *metricspb.Metric, mtpOp internalOperation) {
	var operationLabelIdx int
	for idx, label := range metric.MetricDescriptor.LabelKeys {
		if label.Key == mtpOp.configOperation.Label {
			operationLabelIdx = idx
			break
		}
	}

	groupedTimeseries, unchangedTimeseries := mtp.groupTimeseriesByNewLabelValue(metric, operationLabelIdx, mtpOp.configOperation.NewValue, mtpOp.aggregatedValuesSet)
	aggregatedTimeseries := mtp.mergeTimeseries(groupedTimeseries, mtpOp.configOperation.AggregationType, metric.MetricDescriptor.Type)
	aggregatedTimeseries = append(aggregatedTimeseries, unchangedTimeseries...)

	mtp.sortTimeseries(aggregatedTimeseries)
	metric.Timeseries = aggregatedTimeseries
}

// groupTimeseriesByNewLabelValue groups all timeseries in the metric that will be aggregated together based on the entire label values after replacing the aggregatedValues by newValue.
// Returns a map of grouped timeseries and the corresponding updated label values, as well as a slice of unchanged timeseries
func (mtp *metricsTransformProcessor) groupTimeseriesByNewLabelValue(metric *metricspb.Metric, labelIdx int, newValue string, aggregatedValuesSet map[string]bool) (map[string]*timeseriesAndLabelValues, []*metricspb.TimeSeries) {
	unchangedTimeseries := make([]*metricspb.TimeSeries, 0)
	// key is a composite of the label values as a single string
	groupedTimeseries := make(map[string]*timeseriesAndLabelValues)
	for _, timeseries := range metric.Timeseries {
		if mtp.isUnchangedTimeseries(timeseries, labelIdx, aggregatedValuesSet) {
			unchangedTimeseries = append(unchangedTimeseries, timeseries)
			continue
		}

		key, newLabelValues := mtp.newLabelValuesAsKey(labelIdx, timeseries, newValue, aggregatedValuesSet)
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
	return groupedTimeseries, unchangedTimeseries
}

// isUnchangedTimeseries determines if the timeseries will remain unchanged based on the specified aggregated_values
func (mtp *metricsTransformProcessor) isUnchangedTimeseries(timeseries *metricspb.TimeSeries, labelIdx int, aggregatedValuesSet map[string]bool) bool {
	if _, ok := aggregatedValuesSet[timeseries.LabelValues[labelIdx].Value]; ok {
		return false
	}
	return true
}

// newLabelValuesAsKey composes the key for the timeseries with the aggregatedValues replaced by the newValue
// Returns the key and a slice of the actual values used in this key
func (mtp *metricsTransformProcessor) newLabelValuesAsKey(labelIdx int, timeseries *metricspb.TimeSeries, newValue string, aggregatedValuesSet map[string]bool) (string, []*metricspb.LabelValue) {
	var key string
	newLabelValues := make([]*metricspb.LabelValue, len(timeseries.LabelValues))
	for i, labelValue := range timeseries.LabelValues {
		if _, ok := aggregatedValuesSet[labelValue.Value]; labelIdx == i && ok {
			key += fmt.Sprintf("%v-", newValue)
			newLabelValues[i] = &metricspb.LabelValue{
				Value:    newValue,
				HasValue: labelValue.HasValue,
			}
		} else {
			key += fmt.Sprintf("%v-", labelValue.Value)
			newLabelValues[i] = labelValue
		}
	}
	return key, newLabelValues
}
