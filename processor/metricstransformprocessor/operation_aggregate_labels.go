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
