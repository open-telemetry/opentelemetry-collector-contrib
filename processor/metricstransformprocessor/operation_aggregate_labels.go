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

// aggregateOp aggregates the data points in metric based on given operation
func (mtp *metricsTransformProcessor) aggregateLabelsOp(metric *metricspb.Metric, mtpOp mtpOperation) {
	op := mtpOp.configOperation
	labelSet := mtpOp.labelSetMap
	// labelIdxs is a slice containing the indices of the selected labels.
	// This is needed because label values are ordered in the same order as the labels
	labelIdxs, labels := mtp.getLabelIdxs(metric, labelSet)
	timeseriesGroup := mtp.groupTimeseriesByLabelSet(metric, labelIdxs)

	// merge groups of timeseries
	newTimeseries := mtp.aggregateTimeseriesGroups(timeseriesGroup, op.AggregationType, metric.MetricDescriptor.Type)
	metric.Timeseries = newTimeseries
	metric.MetricDescriptor.LabelKeys = labels
}

// groupTimeseries groups all timeseries in the metric that will be aggregated together based on the selected labels' values indicated by labelIdxs OR on the entire label values after replacing the aggregatedValues by newValue.
// Depending on if newValue is set, the approach to group timeseries is different.
// Returns a map from keys to groups of timeseries and a map from keys to the corresponding label values
func (mtp *metricsTransformProcessor) groupTimeseriesByLabelSet(metric *metricspb.Metric, labelIdxs []int) map[string]*timeseriesGroupByLabelValues {
	// key is a composite of the label values as a single string
	keyToTimeseriesMap := make(map[string]*timeseriesGroupByLabelValues)
	for _, timeseries := range metric.Timeseries {
		var composedValues string
		var newLabelValues []*metricspb.LabelValue
		composedValues, newLabelValues = mtp.composeKeyWithSelectedLabels(labelIdxs, timeseries)
		// group the timeseries together if their values match
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

// composeKeyWithSelectedLabels composes the key for the timeseries based on the selected labels' values indicated by labelIdxs
// Returns the key and a slice of the actual values used in this key
func (mtp *metricsTransformProcessor) composeKeyWithSelectedLabels(labelIdxs []int, timeseries *metricspb.TimeSeries) (string, []*metricspb.LabelValue) {
	var key string
	// newLabelValues are the label values after aggregation, dropping the excluded ones
	newLabelValues := make([]*metricspb.LabelValue, len(labelIdxs))
	for i, vidx := range labelIdxs {
		key += fmt.Sprintf("%v-", timeseries.LabelValues[vidx].Value)
		newLabelValues[i] = timeseries.LabelValues[vidx]
	}
	return key, newLabelValues
}
