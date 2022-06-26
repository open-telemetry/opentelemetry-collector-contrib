// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package serialization // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/dynatraceexporter/internal/serialization"

import (
	"errors"

	dtMetric "github.com/dynatrace-oss/dynatrace-metric-utils-go/metric"
	"github.com/dynatrace-oss/dynatrace-metric-utils-go/metric/dimensions"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func serializeHistogram(name, prefix string, dims dimensions.NormalizedDimensionList, t pmetric.MetricAggregationTemporality, dp pmetric.HistogramDataPoint) (string, error) {
	if t == pmetric.MetricAggregationTemporalityCumulative {
		// convert to delta histogram
		// skip first point because there is nothing to calculate a delta from
		// what if bucket bounds change
		// TTL for cumulative histograms
		// reset detection? if cumulative and count decreases, the process probably reset
		return "", errors.New("cumulative histograms not supported")
	}

	if dp.Count() == 0 {
		return "", nil
	}

	min, max, sum := histDataPointToSummary(dp)

	dm, err := dtMetric.NewMetric(
		name,
		dtMetric.WithPrefix(prefix),
		dtMetric.WithDimensions(dims),
		dtMetric.WithTimestamp(dp.Timestamp().AsTime()),
		dtMetric.WithFloatSummaryValue(min, max, sum, int64(dp.Count())),
	)

	if err != nil {
		return "", err
	}

	return dm.Serialize()
}

// histDataPointToSummary returns the estimated minimum and maximum value in the histogram by using the min and max non-empty buckets.
// It MAY NOT be called with a data point with dp.Count() == 0.
func histDataPointToSummary(dp pmetric.HistogramDataPoint) (float64, float64, float64) {
	bounds := dp.ExplicitBounds()
	counts := dp.BucketCounts()

	// shortcut if min, max, and sum are provided
	if dp.HasMin() && dp.HasMax() && dp.HasSum() {
		return dp.Min(), dp.Max(), dp.Sum()
	}

	// a single-bucket histogram is a special case
	if counts.Len() == 1 {
		return estimateSingleBucketHistogram(dp)
	}

	// If any of min, max, sum is not provided in the data point,
	// loop through the buckets to estimate them.
	// All three values are estimated in order to avoid looping multiple times
	// or complicating the loop with branches. After the loop, estimates
	// will be overridden with any values provided by the data point.
	foundNonEmptyBucket := false
	var min, max, sum float64 = 0, 0, 0

	// Because we do not know the actual min, max, or sum, we estimate them based on non-empty buckets
	for i := 0; i < counts.Len(); i++ {
		// empty bucket
		if counts.At(i) == 0 {
			continue
		}

		// range for bucket counts[i] is bounds[i-1] to bounds[i]

		// min estimation
		if !foundNonEmptyBucket {
			foundNonEmptyBucket = true
			if i == 0 {
				// if we're in the first bucket, the best estimate we can make for min is the upper bound
				min = bounds.At(i)
			} else {
				min = bounds.At(i - 1)
			}
		}

		// max estimation
		if i == counts.Len()-1 {
			// if we're in the last bucket, the best estimate we can make for max is the lower bound
			max = bounds.At(i - 1)
		} else {
			max = bounds.At(i)
		}

		// sum estimation
		switch i {
		case 0:
			// in the first bucket, estimate sum using the upper bound
			sum += float64(counts.At(i)) * bounds.At(i)
		case counts.Len() - 1:
			// in the last bucket, estimate sum using the lower bound
			sum += float64(counts.At(i)) * bounds.At(i-1)
		default:
			// in any other bucket, estimate sum using the bucket midpoint
			sum += float64(counts.At(i)) * (bounds.At(i) + bounds.At(i-1)) / 2
		}
	}

	// Override estimates with any values provided by the data point
	if dp.HasMin() {
		min = dp.Min()
	}
	if dp.HasMax() {
		max = dp.Max()
	}
	if dp.HasSum() {
		sum = dp.Sum()
	}

	// Set min to average when higher than average. This can happen when most values are lower than first boundary (falling in first bucket).
	// Set max to average when lower than average. This can happen when most values are higher than last boundary (falling in last bucket).
	// dp.Count() will never be zero
	avg := sum / float64(dp.Count())
	if min > avg {
		min = avg
	}
	if max < avg {
		max = avg
	}

	return min, max, sum
}

func estimateSingleBucketHistogram(dp pmetric.HistogramDataPoint) (float64, float64, float64) {
	min, max, sum := 0.0, 0.0, 0.0

	if dp.HasSum() {
		sum = dp.Sum()
	}

	mean := sum / float64(dp.Count())

	if dp.HasMin() {
		min = dp.Min()
	} else {
		min = mean
	}

	if dp.HasMax() {
		max = dp.Max()
	} else {
		max = mean
	}

	return min, max, sum
}
