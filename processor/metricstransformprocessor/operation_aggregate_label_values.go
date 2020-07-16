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

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
)

// aggregateLabelValuesOp aggregates the data points in metric across label values
func (mtp *metricsTransformProcessor) aggregateLabelValuesOp(metric *metricspb.Metric, mtpOp mtpOperation) {
	op := mtpOp.configOperation
	labelSet := mtpOp.labelSetMap
	labelIdxs, _ := mtp.getLabelIdxs(metric, labelSet)
	timeseriesGroup := mtp.groupTimeseriesWithNewValue(metric, labelIdxs, op.NewValue, mtpOp.aggregatedValuesSet)
	newTimeseries := mtp.aggregateTimeseriesGroups(timeseriesGroup, op.AggregationType, metric.MetricDescriptor.Type)
	metric.Timeseries = newTimeseries
}

// groupTimeseriesWithNewValue groups all timeseries in the metric that will be aggregated together based on the entire label values after replacing the aggregatedValues by newValue.
// Returns a map from keys to groups of timeseries and the corresponding label values
func (mtp *metricsTransformProcessor) groupTimeseriesWithNewValue(metric *metricspb.Metric, labelIdxs []int, newValue string, aggregatedValuesSet map[string]bool) map[string]*timeseriesGroupByLabelValues {
	// key is a composite of the label values as a single string
	keyToTimeseriesMap := make(map[string]*timeseriesGroupByLabelValues)
	for _, timeseries := range metric.Timeseries {
		var composedValues string
		var newLabelValues []*metricspb.LabelValue
		composedValues, newLabelValues = mtp.composeKeyWithNewValue(labelIdxs[0], timeseries, newValue, aggregatedValuesSet)

		timeseriesGroup, ok := keyToTimeseriesMap[composedValues]
		if ok {
			timeseriesGroup.timeseries = append(timeseriesGroup.timeseries, timeseries)
		} else {
			keyToTimeseriesMap[composedValues] = &timeseriesGroupByLabelValues{
				timeseries:  []*metricspb.TimeSeries{timeseries},
				labelValues: newLabelValues,
			}
		}
	}
	return keyToTimeseriesMap
}

// composeKeyWithNewValue composes the key for the timeseries with the aggregatedValues replaced by the newValue
// Returns the key and a slice of the actual values used in this key
func (mtp *metricsTransformProcessor) composeKeyWithNewValue(labelIdx int, timeseries *metricspb.TimeSeries, newValue string, aggregatedValuesSet map[string]bool) (string, []*metricspb.LabelValue) {
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
