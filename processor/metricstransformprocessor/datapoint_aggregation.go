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
	"math/rand"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	"github.com/golang/protobuf/ptypes/timestamp"
)

// timeseriesGroupByLabelValues is a data structure for grouping timeseries that will be aggregated
type timeseriesGroupByLabelValues struct {
	timeseries  []*metricspb.TimeSeries
	labelValues []*metricspb.LabelValue
}

// aggregateTimeseriesGroups attempts to merge each group of timeseries in keyToTimeseriesMap through the specific calculation indicated by aggrType and dataType to construct the new timeseries with new label values indicated in keyToLabelValuesMap
// Returns the new list of timeseries
func (mtp *metricsTransformProcessor) aggregateTimeseriesGroups(keyToTimeseriesMap map[string]*timeseriesGroupByLabelValues, aggrType AggregationType, dataType metricspb.MetricDescriptor_Type) []*metricspb.TimeSeries {
	newTimeSeriesList := make([]*metricspb.TimeSeries, len(keyToTimeseriesMap))
	idxCounter := 0
	for _, element := range keyToTimeseriesMap {
		timestampToPoints, startTimestamp := mtp.groupPointsForAggregation(element.timeseries)
		newPoints := mtp.aggregatePoints(timestampToPoints, aggrType, dataType)
		newSingleTimeSeries := &metricspb.TimeSeries{
			StartTimestamp: startTimestamp,
			LabelValues:    element.labelValues,
			Points:         newPoints,
		}
		newTimeSeriesList[idxCounter] = newSingleTimeSeries
		idxCounter++
	}

	return newTimeSeriesList
}

// groupPointsForAggregation uses timestamp as an indicator to group aggregatable points in the group of timeseries, and determines the final value of the start timestamp for this groups of timeseries.
// Returns a map that groups points by timestamps and the final start timestamp
func (mtp *metricsTransformProcessor) groupPointsForAggregation(timeseries []*metricspb.TimeSeries) (map[int64][]*metricspb.Point, *timestamp.Timestamp) {
	var startTimestamp *timestamp.Timestamp
	timestampToPoints := make(map[int64][]*metricspb.Point)
	for _, ts := range timeseries {
		// picking the earliest start timestamp for this set of aggregated timeseries
		if startTimestamp == nil || ts.StartTimestamp.Seconds < startTimestamp.Seconds || (ts.StartTimestamp.Seconds == startTimestamp.Seconds && ts.StartTimestamp.Nanos < startTimestamp.Nanos) {
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

// aggregatePoints aggregates points in the groups provided in timestampToPoints by the specific caluculation indicated by aggrType and dataType
// Returns a group of aggregated points
func (mtp *metricsTransformProcessor) aggregatePoints(timestampToPoints map[int64][]*metricspb.Point, aggrType AggregationType, dataType metricspb.MetricDescriptor_Type) []*metricspb.Point {
	newPoints := make([]*metricspb.Point, 0, len(timestampToPoints))
	for _, points := range timestampToPoints {
		// use the timestamp from the first element because these all have the same timestamp as they are grouped by timestamp
		timestamp := points[0].Timestamp
		switch dataType {
		case metricspb.MetricDescriptor_CUMULATIVE_INT64, metricspb.MetricDescriptor_GAUGE_INT64:
			intPoint := mtp.computeInt64(points, aggrType)
			newPoints = append(newPoints, &metricspb.Point{
				Timestamp: timestamp,
				Value:     intPoint,
			})
		case metricspb.MetricDescriptor_CUMULATIVE_DOUBLE, metricspb.MetricDescriptor_GAUGE_DOUBLE:
			doublePoint := mtp.computeDouble(points, aggrType)
			newPoints = append(newPoints, &metricspb.Point{
				Timestamp: timestamp,
				Value:     doublePoint,
			})
		case metricspb.MetricDescriptor_CUMULATIVE_DISTRIBUTION, metricspb.MetricDescriptor_GAUGE_DISTRIBUTION:
			if aggrType != Sum {
				mtp.logger.Warn("Distribution data can only be aggregated by taking the sum")
				break
			}
			distPoints := mtp.computeDistribution(points)
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

// computeInt64 attempts to merge int64 points into one point based on the provided aggrType
// Returns the aggregated int64 value
func (mtp *metricsTransformProcessor) computeInt64(points []*metricspb.Point, aggrType AggregationType) *metricspb.Point_Int64Value {
	intVal := int64(0)
	if len(points) > 0 {
		intVal = points[0].GetInt64Value()
	}
	for _, p := range points[1:] {
		if aggrType == Sum || aggrType == Mean {
			intVal += p.GetInt64Value()
		} else if aggrType == Max {
			intVal = int64(math.Max(float64(intVal), float64(p.GetInt64Value())))
		} else if aggrType == Min {
			intVal = int64(math.Min(float64(intVal), float64(p.GetInt64Value())))
		}
	}
	if aggrType == Mean {
		intVal /= int64(len(points))
	}
	return &metricspb.Point_Int64Value{Int64Value: intVal}
}

// computeDouble attempts to merge double points into one point based on the provided aggrType
// Returns the aggregated double value
func (mtp *metricsTransformProcessor) computeDouble(points []*metricspb.Point, aggrType AggregationType) *metricspb.Point_DoubleValue {
	doubleVal := float64(0)
	if len(points) > 0 {
		doubleVal = points[0].GetDoubleValue()
	}
	for _, p := range points[1:] {
		if aggrType == Sum || aggrType == Mean {
			doubleVal += p.GetDoubleValue()
		} else if aggrType == Max {
			doubleVal = math.Max(doubleVal, p.GetDoubleValue())
		} else if aggrType == Min {
			doubleVal = math.Min(doubleVal, p.GetDoubleValue())
		}
	}
	if aggrType == Mean {
		doubleVal /= float64(len(points))
	}
	return &metricspb.Point_DoubleValue{DoubleValue: doubleVal}
}

// computeDistribution attempts to merge distribution points into one point
// Returns the aggregated distribution value or values if there are mismatching bounds
func (mtp *metricsTransformProcessor) computeDistribution(points []*metricspb.Point) []*metricspb.Point_DistributionValue {
	distVals := make([]*metricspb.Point_DistributionValue, 0)
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
	return distVals
}

// groupPointsByBounds groups distribution value points by the bounds to further determine the aggregatability because distribution value points can only be aggregated if they have match bounds
// Returns a map with groups of aggregatable distribution value points
func (mtp *metricsTransformProcessor) groupPointsByBounds(points []*metricspb.Point) map[string][]*metricspb.Point {
	boundsToPoints := make(map[string][]*metricspb.Point)
	for _, p := range points {
		boundsKey := mtp.boundsToString(p.GetDistributionValue().BucketOptions.GetExplicit().Bounds)
		if groupedPoints, ok := boundsToPoints[boundsKey]; ok {
			boundsToPoints[boundsKey] = append(groupedPoints, p)
		} else {
			boundsToPoints[boundsKey] = []*metricspb.Point{p}
		}
	}
	if len(boundsToPoints) > 1 {
		mtp.logger.Warn("Distribution points with different bounds cannot be aggregated")
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
		Count:                 val1.Count + val2.Count,
		Sum:                   val1.Sum + val2.Sum,
		Buckets:               buckets,
		SumOfSquaredDeviation: mtp.computeSumOfSquaredDeviation(val1, val2),
	}
	return newDistVal
}

// computeSumOfSquaredDeviation computes the combined SumOfSquaredDeviation from the two point groups
// Formula derived and extended from https://math.stackexchange.com/questions/2971315/how-do-i-combine-standard-deviations-of-two-groups
// SSDcomb = SSD1 + n(ave(x) - ave(z))^2 + SSD2 +n(ave(y) - ave(z))^2
func (mtp *metricsTransformProcessor) computeSumOfSquaredDeviation(val1 *metricspb.DistributionValue, val2 *metricspb.DistributionValue) float64 {
	mean1 := val1.Sum / float64(val1.Count)
	mean2 := val2.Sum / float64(val2.Count)
	meanCombined := (val1.Sum + val2.Sum) / float64(val1.Count+val2.Count)
	squaredMeanDiff1 := math.Pow(mean1-meanCombined, 2)
	squaredMeanDiff2 := math.Pow(mean2-meanCombined, 2)
	return val1.SumOfSquaredDeviation + (float64(val1.Count) * squaredMeanDiff1) + val2.SumOfSquaredDeviation + (float64(val2.Count) * squaredMeanDiff2)
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
