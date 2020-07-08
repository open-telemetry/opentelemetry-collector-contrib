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
	"math"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"github.com/golang/protobuf/ptypes/timestamp"
)

// update updates the metric content based on operations indicated in transform.
func (mtp *metricsTransformProcessor) update(metric *metricspb.Metric, transform Transform) {
	// metric name update
	if transform.NewName != "" {
		metric.MetricDescriptor.Name = transform.NewName
	}

	for _, op := range transform.Operations {
		switch op.Action {
		case UpdateLabel:
			mtp.updateLabelOp(metric, op)
		case AggregateLabels, AggregateLabelValues:
			mtp.aggregateOp(metric, op)
		}
	}
}

// updateLabelOp updates labels and label values in metric based on given operation
func (mtp *metricsTransformProcessor) updateLabelOp(metric *metricspb.Metric, op Operation) {
	for idx, label := range metric.MetricDescriptor.LabelKeys {
		if label.Key != op.Label {
			continue
		}
		// label key update
		if op.NewLabel != "" {
			label.Key = op.NewLabel
		}
		// label value update
		labelValuesMapping := mtp.createLabelValueMapping(op.ValueActions)
		for _, timeseries := range metric.Timeseries {
			newValue, ok := labelValuesMapping[timeseries.LabelValues[idx].Value]
			if ok {
				timeseries.LabelValues[idx].Value = newValue
			}
		}
	}
}

// aggregateOp aggregates the data points in metric based on given operation
// This is shared by aggregation across labels and aggregation across label values as they have similar aggragation processes
func (mtp *metricsTransformProcessor) aggregateOp(metric *metricspb.Metric, op Operation) {
	// labelSet is a set of labels to select
	var labelSet map[string]bool
	if op.Action == AggregateLabels {
		labelSet = mtp.sliceToSet(op.LabelSet)
	} else {
		labelSet = map[string]bool{op.Label: true}
	}
	// labelIdxs is a slice containing the indices of the selected labels.
	// This is needed because label values are ordered in the same order as the labels
	labelIdxs, labels := mtp.getLabelIdxs(metric, labelSet)
	// key is a composite of the label values as a single string
	// keyToTimeseriesMap groups timeseries by the label values
	// keyToLabelValuesMap maps the keys to the actual label values objects
	keyToTimeseriesMap, keyToLabelValuesMap := mtp.groupTimeseries(metric, labelIdxs, op.NewValue, op.AggregatedValues)
	// merge groups of timeseries
	newTimeseries := mtp.aggregateTimeseriesGroups(keyToTimeseriesMap, keyToLabelValuesMap, op.AggregationType, metric.MetricDescriptor.Type)
	metric.Timeseries = newTimeseries
	if op.Action == AggregateLabels {
		metric.MetricDescriptor.LabelKeys = labels
	}
}

// createLabelValueMapping creates a label value mapping from old value to new value based on valueActions
// Returns the mapping
func (mtp *metricsTransformProcessor) createLabelValueMapping(valueActions []ValueAction) map[string]string {
	mapping := make(map[string]string)
	for _, valueAction := range valueActions {
		mapping[valueAction.Value] = valueAction.NewValue
	}
	return mapping
}

// sliceToSet converts slice of strings to set of strings
// Returns the set of strings
func (mtp *metricsTransformProcessor) sliceToSet(slice []string) map[string]bool {
	set := make(map[string]bool, len(slice))
	for _, label := range slice {
		set[label] = true
	}
	return set
}

// getLabelIdxs gets the indices of the labelSet labels' indices in the metric's descriptor's labels field
// Returns the indices slice and a slice of the actual labels selected by this slice of indices
func (mtp *metricsTransformProcessor) getLabelIdxs(metric *metricspb.Metric, labelSet map[string]bool) ([]int, []*metricspb.LabelKey) {
	labelIdxs := make([]int, 0)
	labels := make([]*metricspb.LabelKey, 0)
	for idx, label := range metric.MetricDescriptor.LabelKeys {
		_, ok := labelSet[label.Key]
		if ok {
			labelIdxs = append(labelIdxs, idx)
			labels = append(labels, label)
		}
	}
	return labelIdxs, labels
}

// groupTimeseries groups all timeseries in the metric that will be aggregated together based on the selected labels' values indicated by labelIdxs OR on the entire label values after replacing the aggregatedValues by newValue.
// Depending on if newValue is set, the approach to group timeseries is different.
// Returns a map from keys to groups of timeseries and a map from keys to the corresponding label values
func (mtp *metricsTransformProcessor) groupTimeseries(metric *metricspb.Metric, labelIdxs []int, newValue string, aggregatedValues []string) (map[string][]*metricspb.TimeSeries, map[string][]*metricspb.LabelValue) {
	// key is a composite of the label values as a single string
	// keyToTimeseriesMap groups timeseries by the label values
	keyToTimeseriesMap := make(map[string][]*metricspb.TimeSeries)
	// keyToLabelValuesMap maps the keys to the actual label values objects
	keyToLabelValuesMap := make(map[string][]*metricspb.LabelValue)
	for _, timeseries := range metric.Timeseries {
		var composedValues string
		var newLabelValues []*metricspb.LabelValue
		if newValue == "" {
			composedValues, newLabelValues = mtp.composeKeyWithSelectedLabels(labelIdxs, timeseries)
		} else {
			composedValues, newLabelValues = mtp.composeKeyWithNewValue(labelIdxs[0], timeseries, newValue, aggregatedValues)
		}
		// group the timeseries together if their values match
		timeseriesGroup, ok := keyToTimeseriesMap[composedValues]
		if ok {
			keyToTimeseriesMap[composedValues] = append(timeseriesGroup, timeseries)
		} else {
			keyToTimeseriesMap[composedValues] = []*metricspb.TimeSeries{timeseries}
			keyToLabelValuesMap[composedValues] = newLabelValues
		}
	}
	return keyToTimeseriesMap, keyToLabelValuesMap
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

// composeKeyWithNewValue composes the key for the timeseries with the aggregatedValues replaced by the newValue
// Returns the key and a slice of the actual values used in this key
func (mtp *metricsTransformProcessor) composeKeyWithNewValue(labelIdx int, timeseries *metricspb.TimeSeries, newValue string, aggregatedValues []string) (string, []*metricspb.LabelValue) {
	aggregatredValuesSet := mtp.sliceToSet(aggregatedValues)
	var key string
	// newLabelValues are the label values after aggregation with the new value name
	newLabelValues := make([]*metricspb.LabelValue, len(timeseries.LabelValues))
	for i, labelValue := range timeseries.LabelValues {
		if _, ok := aggregatredValuesSet[labelValue.Value]; labelIdx == i && ok {
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

// aggregateTimeseriesGroups attempts to merge each group of timesreis in keyToTimeseriesMap through the spefic calculation indicated by aggrType and dataType to construct the new timeseries with new label values indicated in keyToLabelValuesMap
// Returns the new list of timeseries
func (mtp *metricsTransformProcessor) aggregateTimeseriesGroups(keyToTimeseriesMap map[string][]*metricspb.TimeSeries, keyToLabelValuesMap map[string][]*metricspb.LabelValue, aggrType AggregationType, dataType metricspb.MetricDescriptor_Type) []*metricspb.TimeSeries {
	newTimeSeriesList := make([]*metricspb.TimeSeries, len(keyToTimeseriesMap))
	idxCounter := 0
	for key, element := range keyToTimeseriesMap {
		timestampToPoints, startTimestamp := mtp.analyzeTimeseriesForAggregation(element)
		newPoints := mtp.aggregatePoints(timestampToPoints, aggrType, dataType)
		// newSingleTimeSeries is an aggregated timeseries
		newSingleTimeSeries := &metricspb.TimeSeries{
			StartTimestamp: startTimestamp,
			LabelValues:    keyToLabelValuesMap[key],
			Points:         newPoints,
		}
		newTimeSeriesList[idxCounter] = newSingleTimeSeries
		idxCounter++
	}

	return newTimeSeriesList
}

// analyzeTimeseriesForAggregation uses timestamp as an indicator to group aggregatable points in the group of timeseries, and determines the final value of the start timestamp for this groups of timeseries.
// Returns a map that groups points by timestamps and the final start timestamp
func (mtp *metricsTransformProcessor) analyzeTimeseriesForAggregation(timeseries []*metricspb.TimeSeries) (map[string][]*metricspb.Point, *timestamp.Timestamp) {
	var startTimestamp *timestamp.Timestamp
	// timestampToPoints maps from timestamp string to aggregatable points
	timestampToPoints := make(map[string][]*metricspb.Point)
	for _, ts := range timeseries {
		if startTimestamp == nil || ts.StartTimestamp.Seconds < startTimestamp.Seconds || (ts.StartTimestamp.Seconds == startTimestamp.Seconds && ts.StartTimestamp.Nanos < startTimestamp.Nanos) {
			startTimestamp = ts.StartTimestamp
		}
		for _, p := range ts.Points {
			if points, ok := timestampToPoints[p.Timestamp.String()]; ok {
				timestampToPoints[p.Timestamp.String()] = append(points, p)
			} else {
				timestampToPoints[p.Timestamp.String()] = []*metricspb.Point{p}
			}
		}
	}
	return timestampToPoints, startTimestamp
}

// aggregatePoints aggregates points in the groups provided in timestampToPoints by the specific caluculation indicated by aggrType and dataType
// Returns a group of aggregated points
func (mtp *metricsTransformProcessor) aggregatePoints(timestampToPoints map[string][]*metricspb.Point, aggrType AggregationType, dataType metricspb.MetricDescriptor_Type) []*metricspb.Point {
	// initialize to size 0. unsure how many points will come out because distribution points are not guaranteed to be aggregatable. (mismatched bounds)
	newPoints := make([]*metricspb.Point, 0)
	for _, points := range timestampToPoints {
		// use the timestamp from the first element because these all have the same timestamp as they are grouped by timestamp
		timestamp := points[0].Timestamp
		intPoint, doublePoint, distPoints := mtp.compute(points, aggrType, dataType)
		if intPoint != nil {
			newPoints = append(newPoints, &metricspb.Point{
				Timestamp: timestamp,
				Value:     intPoint,
			})
		} else if doublePoint != nil {
			newPoints = append(newPoints, &metricspb.Point{
				Timestamp: timestamp,
				Value:     doublePoint,
			})
		} else if distPoints != nil {
			for _, p := range distPoints {
				newPoints = append(newPoints, &metricspb.Point{
					Timestamp: timestamp,
					Value:     p,
				})
			}
		}
	}
	return newPoints
}

// compute attempts to merge points into one point based on the provided aggrType and dataType
// Returns an int value, a double value and a slice of distribution values, and only one of these would contain values based on the dataType.
func (mtp *metricsTransformProcessor) compute(points []*metricspb.Point, aggrType AggregationType, dataType metricspb.MetricDescriptor_Type) (*metricspb.Point_Int64Value, *metricspb.Point_DoubleValue, []*metricspb.Point_DistributionValue) {
	intVal := int64(0)
	doubleVal := float64(0)
	distVals := make([]*metricspb.Point_DistributionValue, 0)
	switch dataType {
	case metricspb.MetricDescriptor_GAUGE_INT64, metricspb.MetricDescriptor_CUMULATIVE_INT64:
		for _, p := range points {
			if aggrType == Sum || aggrType == Average {
				intVal += p.GetInt64Value()
			} else if aggrType == Max {
				if p.GetInt64Value() <= intVal {
					continue
				}
				intVal = p.GetInt64Value()
			}

		}
		if aggrType == Average {
			intVal /= int64(len(points))
		}
		return &metricspb.Point_Int64Value{Int64Value: intVal}, nil, nil
	case metricspb.MetricDescriptor_GAUGE_DOUBLE, metricspb.MetricDescriptor_CUMULATIVE_DOUBLE:
		for _, p := range points {
			if aggrType == Sum || aggrType == Average {
				doubleVal += p.GetDoubleValue()
			} else if aggrType == Max {
				if p.GetDoubleValue() <= doubleVal {
					continue
				}
				doubleVal = p.GetDoubleValue()
			}

		}
		if aggrType == Average {
			doubleVal /= float64(len(points))
		}
		return nil, &metricspb.Point_DoubleValue{DoubleValue: doubleVal}, nil
	case metricspb.MetricDescriptor_GAUGE_DISTRIBUTION, metricspb.MetricDescriptor_CUMULATIVE_DISTRIBUTION:
		if aggrType != Sum {
			break
		}
		boundsToPoints := mtp.groupPointsByBounds(points)
		for _, v := range boundsToPoints {
			var distVal *metricspb.DistributionValue
			for _, p := range v {
				if distVal == nil {
					distVal = p.GetDistributionValue()
					continue
				}
				distVal = mtp.computeDistVals(distVal, p.GetDistributionValue())
			}
			distVals = append(distVals, &metricspb.Point_DistributionValue{DistributionValue: distVal})
		}
		return nil, nil, distVals

	}
	return nil, nil, nil
}

// groupPointsByBounds groups distribution value points by the bounds to further determine the aggregatability because distribution value points can only be aggregated if they have match bounds
// Returns a map with groups of aggregatable distribution value points
func (mtp *metricsTransformProcessor) groupPointsByBounds(points []*metricspb.Point) map[string][]*metricspb.Point {
	boundsToPoints := make(map[string][]*metricspb.Point, 0)
	for _, p := range points {
		boundsKey := mtp.boundsToString(p.GetDistributionValue().BucketOptions.GetExplicit().Bounds)
		if groupedPoints, ok := boundsToPoints[boundsKey]; ok {
			boundsToPoints[boundsKey] = append(groupedPoints, p)
		} else {
			boundsToPoints[boundsKey] = []*metricspb.Point{p}
		}
	}
	return boundsToPoints
}

// boundsToString converts bounds slice into a string
// Returns a string that represents this bounds
func (mtp *metricsTransformProcessor) boundsToString(bounds []float64) string {
	var boundsKey string
	for _, b := range bounds {
		boundsKey += fmt.Sprintf("%f-", b)
	}
	return boundsKey
}

// sumOfSquaredDeviation calculates the combined SumOfSquaredDeviation from the ones in val1 and val2
// Formula derived and extended from https://math.stackexchange.com/questions/2971315/how-do-i-combine-standard-deviations-of-two-groups
// SSDcomb = SSD1 + n(ave(x) - ave(z))^2 + SSD2 +n(ave(y) - ave(z))^2
func (mtp *metricsTransformProcessor) computeDistVals(val1 *metricspb.DistributionValue, val2 *metricspb.DistributionValue) *metricspb.DistributionValue {
	buckets := make([]*metricspb.DistributionValue_Bucket, len(val1.Buckets))
	for i := range buckets {
		buckets[i] = &metricspb.DistributionValue_Bucket{
			Count: val1.Buckets[i].Count + val2.Buckets[i].Count,
		}
	}
	mean1 := val1.Sum / float64(val1.Count)
	mean2 := val2.Sum / float64(val2.Count)
	meanCombined := (val1.Sum + val2.Sum) / float64(val1.Count+val2.Count)
	squaredMeanDiff1 := math.Pow(mean1-meanCombined, 2)
	squaredMeanDiff2 := math.Pow(mean2-meanCombined, 2)
	newDistVal := &metricspb.DistributionValue{
		BucketOptions: &metricspb.DistributionValue_BucketOptions{
			Type: &metricspb.DistributionValue_BucketOptions_Explicit_{
				Explicit: &metricspb.DistributionValue_BucketOptions_Explicit{
					Bounds: val1.BucketOptions.GetExplicit().Bounds,
				},
			},
		},
		Count:                 val1.Count + val2.Count,
		Sum:                   val1.Sum + val2.Sum,
		Buckets:               buckets,
		SumOfSquaredDeviation: val1.SumOfSquaredDeviation + (float64(val1.Count) * squaredMeanDiff1) + val2.SumOfSquaredDeviation + (float64(val2.Count) * squaredMeanDiff2),
	}
	return newDistVal
}
