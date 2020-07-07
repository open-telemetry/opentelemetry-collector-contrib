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
	"math"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"github.com/golang/protobuf/ptypes/timestamp"
)

// update updates the original metric content in the metric pointer.
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

// updateLabelOp updates labels and label values based on given operation
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

// aggregateLabelsOp aggregates the data points in metric across labels
func (mtp *metricsTransformProcessor) aggregateOp(metric *metricspb.Metric, op Operation) {
	// labelSet is a set of labels to keep
	var labelSet map[string]bool
	if op.Action == AggregateLabels {
		labelSet = mtp.sliceToSet(op.LabelSet)
	} else {
		labelSet = map[string]bool{op.Label: true}
	}
	// labelIdxs is a slice containing the indices of the labels to keep.
	// This is needed because label values are ordered in the same order as the labels
	labelIdxs, labels := mtp.getLabelIdxs(metric, labelSet)
	// key is a composite of the label values as a single string
	// keyToTimeseriesMap groups timeseries by the label values
	// keyToLabelValuesMap groups the actual label values objects
	keyToTimeseriesMap, keyToLabelValuesMap := mtp.groupTimeseries(metric, labelIdxs, op.NewValue)
	// compose groups into timeseries
	newTimeseries := mtp.aggregateTimeseriesGroups(keyToTimeseriesMap, keyToLabelValuesMap, op.AggregationType)
	metric.Timeseries = newTimeseries
	if op.Action == AggregateLabels {
		metric.MetricDescriptor.LabelKeys = labels
	}
}

// createLabelValueMapping creates a label value mapping from old value to new value.
func (mtp *metricsTransformProcessor) createLabelValueMapping(valueActions []ValueAction) map[string]string {
	mapping := make(map[string]string)
	for _, valueAction := range valueActions {
		mapping[valueAction.Value] = valueAction.NewValue
	}
	return mapping
}

// sliceToSet converts slice of strings to set of strings
func (mtp *metricsTransformProcessor) sliceToSet(slice []string) map[string]bool {
	set := make(map[string]bool, len(slice))
	for _, label := range slice {
		set[label] = true
	}
	return set
}

// getLabelIdxs gets the indices of the labels in the labelSet in a metric's descriptor
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

// groupTimeseries groups timeseries into groups that will be aggregated together
func (mtp *metricsTransformProcessor) groupTimeseries(metricPtr *metricspb.Metric, labelIdxs []int, newValue string) (map[string][]*metricspb.TimeSeries, map[string][]*metricspb.LabelValue) {
	// key is a composite of the label values as a single string
	// keyToTimeseriesMap groups timeseries by the label values
	keyToTimeseriesMap := make(map[string][]*metricspb.TimeSeries)
	// keyToLabelValuesMap groups the actual label values objects
	keyToLabelValuesMap := make(map[string][]*metricspb.LabelValue)
	for _, timeseries := range metricPtr.Timeseries {
		var composedValues string
		var newLabelValues []*metricspb.LabelValue
		if newValue == "" {
			composedValues, newLabelValues = mtp.composeKeyWithSelectedLabels(labelIdxs, timeseries)
		} else {
			composedValues, newLabelValues = mtp.composeKeyWithNewValue(labelIdxs[0], timeseries, newValue)
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

func (mtp *metricsTransformProcessor) composeKeyWithSelectedLabels(labelIdxs []int, timeseries *metricspb.TimeSeries) (string, []*metricspb.LabelValue) {
	var composedValues string
	// newLabelValues are the label values after aggregation, dropping the excluded ones
	newLabelValues := make([]*metricspb.LabelValue, len(labelIdxs))
	for i, vidx := range labelIdxs {
		composedValues += timeseries.LabelValues[vidx].Value
		newLabelValues[i] = timeseries.LabelValues[vidx]
	}
	return composedValues, newLabelValues
}

func (mtp *metricsTransformProcessor) composeKeyWithNewValue(labelIdx int, timeseries *metricspb.TimeSeries, newValue string) (string, []*metricspb.LabelValue) {
	var composedValues string
	// newLabelValues are the label values after aggregation with the new value name
	newLabelValues := make([]*metricspb.LabelValue, len(timeseries.LabelValues))
	for i, labelValue := range timeseries.LabelValues {
		if labelIdx == i {
			composedValues += newValue
			newLabelValues[i] = &metricspb.LabelValue{
				Value:    newValue,
				HasValue: labelValue.HasValue,
			}
		} else {
			composedValues += labelValue.Value
			newLabelValues[i] = labelValue
		}
	}
	return composedValues, newLabelValues
}

// aggregateTimeseriesGroups merges each group into one timeseries
func (mtp *metricsTransformProcessor) aggregateTimeseriesGroups(keyToTimeseriesMap map[string][]*metricspb.TimeSeries, keyToLabelValuesMap map[string][]*metricspb.LabelValue, aggrType AggregationType) []*metricspb.TimeSeries {
	newTimeSeriesList := make([]*metricspb.TimeSeries, len(keyToTimeseriesMap))
	idxCounter := 0
	for key, element := range keyToTimeseriesMap {
		timestampToPoints, startTimestamp := mtp.analyzeTimeseriesForAggregation(element)
		newPoints := make([]*metricspb.Point, len(timestampToPoints))
		pidxCounter := 0
		for _, points := range timestampToPoints {
			newPoints[pidxCounter] = &metricspb.Point{
				Timestamp: points[0].Timestamp,
			}
			intPoint, doublePoint, distPoint := mtp.compute(points, aggrType)
			if intPoint != nil {
				newPoints[pidxCounter].Value = intPoint
			} else if doublePoint != nil {
				newPoints[pidxCounter].Value = doublePoint
			} else {
				newPoints[pidxCounter].Value = distPoint
			}
			pidxCounter++
		}
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

// analyzeTimeseriesForAggregation uses timestamp as an indicator to group aggregatable points, and determines the value for start timestamp.
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

// compute merges points into one point based on the provided aggregation type
// TODO: ensure no dropping data for distributed
func (mtp *metricsTransformProcessor) compute(points []*metricspb.Point, aggrType AggregationType) (*metricspb.Point_Int64Value, *metricspb.Point_DoubleValue, *metricspb.Point_DistributionValue) {
	intVal := int64(0)
	doubleVal := float64(0)
	var distVal *metricspb.DistributionValue
	if points[0].GetInt64Value() != 0 {
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
	} else if points[0].GetDoubleValue() != 0 {
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
	} else if points[0].GetDistributionValue() != nil && aggrType == Sum {
		for _, p := range points {
			if distVal == nil {
				distVal = p.GetDistributionValue()
				continue
			}
			if mtp.haveMatchingBounds(distVal, p.GetDistributionValue()) {
				distVal = mtp.computeDistVals(distVal, p.GetDistributionValue())
			}
		}
		return nil, nil, &metricspb.Point_DistributionValue{DistributionValue: distVal}
	}
	return nil, nil, nil
}

func (mtp *metricsTransformProcessor) haveMatchingBounds(val1 *metricspb.DistributionValue, val2 *metricspb.DistributionValue) bool {
	bounds1 := val1.BucketOptions.GetExplicit().Bounds
	bounds2 := val2.BucketOptions.GetExplicit().Bounds
	if len(bounds1) != len(bounds2) {
		return false
	}
	for i, b := range bounds1 {
		if b != bounds2[i] {
			return false
		}
	}
	return true
}

// sumOfSquaredDeviation calculation
// https://math.stackexchange.com/questions/2971315/how-do-i-combine-standard-deviations-of-two-groups
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
