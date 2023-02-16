// Copyright The OpenTelemetry Authors
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

package metrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/metrics"

import (
	"context"
	"fmt"
	"math"
	"strconv"

	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottldatapoint"
)

func imin(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func imax(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func convertGaugeToHistogram(stringAggTemp string, bucketCount int64, lowerBound float64, upperBound float64) (ottl.ExprFunc[ottldatapoint.TransformContext], error) {
	var aggTemp pmetric.AggregationTemporality
	switch stringAggTemp {
	case "delta":
		aggTemp = pmetric.AggregationTemporalityDelta
	case "cumulative":
		aggTemp = pmetric.AggregationTemporalityCumulative
	default:
		return nil, fmt.Errorf("unknown aggregation temporality: %s", stringAggTemp)
	}
	if bucketCount <= 0 {
		return nil, fmt.Errorf("Bucket value must be greater than 0. Current value: %s", strconv.Itoa(int(bucketCount)))
	}
	if lowerBound > upperBound {
		return nil, fmt.Errorf("Lower bound must be less than or equal to upper bound.")
	}
	return func(_ context.Context, tCtx ottldatapoint.TransformContext) (interface{}, error) {
		metric := tCtx.GetMetric()
		if metric.Type() != pmetric.MetricTypeGauge {
			return nil, nil
		}

		dps := metric.Gauge().DataPoints()
		length := dps.Len()

		metric.SetEmptyHistogram().SetAggregationTemporality(aggTemp)

		if length == 0 {
			return nil, nil
		}

		histogram := metric.Histogram().DataPoints().AppendEmpty()

		dp := dps.At(0)
		histogram.SetStartTimestamp(dp.StartTimestamp())
		histogram.SetTimestamp(dp.Timestamp())

		switch dp.ValueType() {
		case pmetric.NumberDataPointValueTypeDouble:
			min := dp.DoubleValue()
			max := dp.DoubleValue()
			var sum float64 = dp.DoubleValue()

			for i := 1; i < length; i++ {
				if dps.At(i).StartTimestamp() < histogram.StartTimestamp() {
					histogram.SetStartTimestamp(dps.At(i).StartTimestamp())
				}
				if dps.At(i).Timestamp() > histogram.Timestamp() {
					histogram.SetTimestamp(dps.At(i).Timestamp())
				}

				val := dps.At(i).DoubleValue()
				min = math.Min(val, min)
				max = math.Max(val, max)
				sum += val
			}
			histogram.SetMax(max)
			histogram.SetMin(min)
			histogram.SetSum(sum)
			histogram.SetCount(uint64(length))

			if length > 1 {
				histogram.BucketCounts().FromRaw(make([]uint64, bucketCount))
				if lowerBound == upperBound {
					lowerBound = min
					upperBound = max
				}

				scale := float64(upperBound-lowerBound) / float64(bucketCount)

				for i := 0; i < dps.Len(); i++ {
					diff := dps.At(i).DoubleValue() - lowerBound
					float_index := diff / scale
					// if it hits on an exact explicit bound, then move it to the bucket below per histogram spec, as the buckets are upper bound inclusive
					if float_index == float64(int64(float_index)) && int64(float_index) > 0 && int64(float_index) < bucketCount {
						float_index -= 1
					}
					index := int(imin(int64(float_index), bucketCount-1))
					histogram.BucketCounts().SetAt(index, histogram.BucketCounts().At(index)+1)
				}

				for i := 1; i < int(bucketCount); i++ {
					histogram.ExplicitBounds().Append(lowerBound + (scale * float64(i)))
				}
			}
		case pmetric.NumberDataPointValueTypeInt:
			min := dp.IntValue()
			max := dp.IntValue()
			var sum int64 = dp.IntValue()

			for i := 1; i < length; i++ {
				if dps.At(i).StartTimestamp() < histogram.StartTimestamp() {
					histogram.SetStartTimestamp(dps.At(i).StartTimestamp())
				}
				if dps.At(i).Timestamp() > histogram.Timestamp() {
					histogram.SetTimestamp(dps.At(i).Timestamp())
				}
				val := dps.At(i).IntValue()
				min = imin(val, min)
				max = imax(val, max)
				sum += val
			}
			histogram.SetMax(float64(max))
			histogram.SetMin(float64(min))
			histogram.SetSum(float64(sum))
			histogram.SetCount(uint64(length))

			if length > 1 {
				if lowerBound == upperBound {
					lowerBound = float64(min)
					upperBound = float64(max)
				}

				histogram.BucketCounts().FromRaw(make([]uint64, bucketCount))
				scale := float64(upperBound-lowerBound) / float64(bucketCount)

				for i := 0; i < dps.Len(); i++ {
					diff := float64(dps.At(i).IntValue()) - lowerBound
					float_index := diff / scale
					// if it hits on an exact explicit bound, then move it to the bucket below per histogram spec, as the buckets are upper bound inclusive
					if float_index == float64(int64(float_index)) && int64(float_index) > 0 && int64(float_index) < bucketCount {
						float_index -= 1
					}
					index := int(imin(int64(float_index), bucketCount-1))
					histogram.BucketCounts().SetAt(index, histogram.BucketCounts().At(index)+1)
				}

				for i := 1; i < int(bucketCount); i++ {
					histogram.ExplicitBounds().Append(lowerBound + (scale * float64(i)))
				}
			}
		}

		return nil, nil
	}, nil
}
