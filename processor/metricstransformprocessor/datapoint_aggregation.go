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
	"math"
	"math/rand"
	"sort"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// timeseriesAndLabelValues is a data structure for grouping timeseries that will be aggregated
type timeseriesAndLabelValues struct {
	timeseries  []*metricspb.TimeSeries
	labelValues []*metricspb.LabelValue
}

// mergeTimeseries merges each group of timeseries using the specified aggregation
// Returns the merged timeseries
func (mtp *metricsTransformProcessor) mergeTimeseries(groupedTimeseries map[string]*timeseriesAndLabelValues, aggrType AggregationType, metricType metricspb.MetricDescriptor_Type) []*metricspb.TimeSeries {
	newTimeSeriesList := make([]*metricspb.TimeSeries, len(groupedTimeseries))
	idxCounter := 0
	for _, tlv := range groupedTimeseries {
		timestampToPoints, startTimestamp := mtp.groupPointsByTimestamp(tlv.timeseries)
		newPoints := mtp.mergePoints(timestampToPoints, aggrType, metricType)
		newSingleTimeSeries := &metricspb.TimeSeries{
			StartTimestamp: startTimestamp,
			LabelValues:    tlv.labelValues,
			Points:         newPoints,
		}
		newTimeSeriesList[idxCounter] = newSingleTimeSeries
		idxCounter++
	}

	return newTimeSeriesList
}

// sortTimeseries performs an in place sort of a list of timeseries by start timestamp
// Returns the sorted timeseries
func (mtp *metricsTransformProcessor) sortTimeseries(timeseries []*metricspb.TimeSeries) {
	sort.Slice(timeseries, func(i, j int) bool {
		return mtp.compareTimestamps(timeseries[i].StartTimestamp, timeseries[j].StartTimestamp)
	})
}

// groupPointsByTimestamp groups points by timestamp
// Returns a map of grouped points and the minimum start timestamp
func (mtp *metricsTransformProcessor) groupPointsByTimestamp(timeseries []*metricspb.TimeSeries) (map[int64][]*metricspb.Point, *timestamppb.Timestamp) {
	var startTimestamp *timestamppb.Timestamp
	timestampToPoints := make(map[int64][]*metricspb.Point)
	for _, ts := range timeseries {
		if startTimestamp == nil {
			startTimestamp = ts.StartTimestamp
		}
		for _, p := range ts.Points {
			if points, ok := timestampToPoints[p.Timestamp.Seconds]; ok {
				timestampToPoints[p.Timestamp.Seconds] = append(points, p)
			} else {
				timestampToPoints[p.Timestamp.Seconds] = []*metricspb.Point{p}
			}
		}
	}
	return timestampToPoints, startTimestamp
}

// mergePoints aggregates points in the groups provided in timestampToPoints by the specific caluculation indicated by aggrType and dataType
// Returns a group of aggregated points
func (mtp *metricsTransformProcessor) mergePoints(timestampToPoints map[int64][]*metricspb.Point, aggrType AggregationType, metricType metricspb.MetricDescriptor_Type) []*metricspb.Point {
	newPoints := make([]*metricspb.Point, 0, len(timestampToPoints))
	for _, points := range timestampToPoints {
		timestamp := points[0].Timestamp
		switch metricType {
		case metricspb.MetricDescriptor_CUMULATIVE_INT64, metricspb.MetricDescriptor_GAUGE_INT64:
			intPoint := mtp.mergeInt64(points, aggrType)
			newPoints = append(newPoints, &metricspb.Point{
				Timestamp: timestamp,
				Value:     intPoint,
			})
		case metricspb.MetricDescriptor_CUMULATIVE_DOUBLE, metricspb.MetricDescriptor_GAUGE_DOUBLE:
			doublePoint := mtp.mergeDouble(points, aggrType)
			newPoints = append(newPoints, &metricspb.Point{
				Timestamp: timestamp,
				Value:     doublePoint,
			})
		case metricspb.MetricDescriptor_CUMULATIVE_DISTRIBUTION, metricspb.MetricDescriptor_GAUGE_DISTRIBUTION:
			if aggrType != Sum {
				mtp.logger.Warn("Distribution data can only be aggregated by taking the sum")
				newPoints = append(newPoints, points...)
				break
			}
			distPoint := mtp.mergeDistribution(points)
			newPoints = append(newPoints, &metricspb.Point{
				Timestamp: timestamp,
				Value:     distPoint,
			})

		}
	}
	sort.Slice(newPoints, func(i, j int) bool {
		return mtp.compareTimestamps(newPoints[i].Timestamp, newPoints[j].Timestamp)
	})
	return newPoints
}

// mergeInt64 merges int64 points into one point based on the provided aggrType
// Returns the aggregated int64 value
func (mtp *metricsTransformProcessor) mergeInt64(points []*metricspb.Point, aggrType AggregationType) *metricspb.Point_Int64Value {
	intVal := int64(0)
	if len(points) > 0 {
		intVal = points[0].GetInt64Value()
	}
	for _, p := range points[1:] {
		switch aggrType {
		case Sum, Mean:
			intVal += p.GetInt64Value()
		case Max:
			intVal = mtp.maxInt64(intVal, p.GetInt64Value())
		case Min:
			intVal = mtp.minInt64(intVal, p.GetInt64Value())
		}
	}
	if aggrType == Mean {
		intVal /= int64(len(points))
	}
	return &metricspb.Point_Int64Value{Int64Value: intVal}
}

// mergeDouble merges double points into one point based on the provided aggrType
// Returns the aggregated double value
func (mtp *metricsTransformProcessor) mergeDouble(points []*metricspb.Point, aggrType AggregationType) *metricspb.Point_DoubleValue {
	doubleVal := float64(0)
	if len(points) > 0 {
		doubleVal = points[0].GetDoubleValue()
	}
	for _, p := range points[1:] {
		switch aggrType {
		case Sum, Mean:
			doubleVal += p.GetDoubleValue()
		case Max:
			doubleVal = math.Max(doubleVal, p.GetDoubleValue())
		case Min:
			doubleVal = math.Min(doubleVal, p.GetDoubleValue())
		}
	}
	if aggrType == Mean {
		doubleVal /= float64(len(points))
	}
	return &metricspb.Point_DoubleValue{DoubleValue: doubleVal}
}

// mergeDistribution merges distribution points into one point
// Returns the aggregated distribution value
func (mtp *metricsTransformProcessor) mergeDistribution(points []*metricspb.Point) *metricspb.Point_DistributionValue {
	var distVal *metricspb.DistributionValue
	for _, p := range points {
		if distVal == nil {
			distVal = p.GetDistributionValue()
			continue
		}
		distVal = mtp.computeDistVals(distVal, p.GetDistributionValue())
	}
	return &metricspb.Point_DistributionValue{DistributionValue: distVal}
}

// computeDistVals does the necessary computations and aggregation to combine two distribution value into one
// returns the combined distribution value
func (mtp *metricsTransformProcessor) computeDistVals(val1 *metricspb.DistributionValue, val2 *metricspb.DistributionValue) *metricspb.DistributionValue {
	buckets := make([]*metricspb.DistributionValue_Bucket, len(val1.Buckets))
	for i := range buckets {
		buckets[i] = &metricspb.DistributionValue_Bucket{
			Count:    val1.Buckets[i].Count + val2.Buckets[i].Count,
			Exemplar: mtp.pickExemplar(val1.Buckets[i].Exemplar, val2.Buckets[i].Exemplar),
		}
	}
	newDistVal := &metricspb.DistributionValue{
		BucketOptions: &metricspb.DistributionValue_BucketOptions{
			Type: &metricspb.DistributionValue_BucketOptions_Explicit_{
				Explicit: &metricspb.DistributionValue_BucketOptions_Explicit{
					Bounds: val1.BucketOptions.GetExplicit().Bounds,
				},
			},
		},
		Count:   val1.Count + val2.Count,
		Sum:     val1.Sum + val2.Sum,
		Buckets: buckets,
	}
	return newDistVal
}

// pickExemplar picks an exemplar from 2 randomly with each haing a 50% chance of getting picked
func (mtp *metricsTransformProcessor) pickExemplar(e1 *metricspb.DistributionValue_Exemplar, e2 *metricspb.DistributionValue_Exemplar) *metricspb.DistributionValue_Exemplar {
	r := rand.Intn(2)
	pickedExemplar := e1
	if r == 1 {
		pickedExemplar = e2
	}
	return pickedExemplar
}
