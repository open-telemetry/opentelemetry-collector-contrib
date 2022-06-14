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
func histDataPointToSummary(dp pmetric.HistogramDataPoint) (float64, float64, float64) {
	bounds := dp.MExplicitBounds()
	counts := dp.MBucketCounts()

	// shortcut if min, max, and sum are provided
	if dp.HasMin() && dp.HasMax() && dp.HasSum() {
		return dp.Min(), dp.Max(), dp.Sum()
	}

	// a single-bucket histogram is a special case
	if len(counts) == 1 {
		return estimateSingleBucketHistogram(dp)
	}

	foundNonEmptyBucket := false
	var min, max, sum float64 = 0, 0, 0

	// Because we do not know the actual min, max, or sum, we estimate them based on non-empty buckets
	for i := 0; i < len(counts); i++ {
		// empty bucket
		if counts[i] == 0 {
			continue
		}

		// range for counts[i] is bounds[i-1] to bounds[i]

		// min estimation
		if !foundNonEmptyBucket {
			foundNonEmptyBucket = true
			if i == 0 {
				// if we're in the first bucket, the best estimate we can make for min is the upper bound
				min = bounds[i]
			} else {
				min = bounds[i-1]
			}
		}

		if i == len(counts)-1 {
			// if we're in the last bucket, the best estimate we can make for max is the lower bound
			max = bounds[i-1]
		} else {
			max = bounds[i]
		}

		if i == 0 {
			// in the first bucket, estimate sum using the upper bound
			sum += float64(counts[i]) * bounds[i]
		} else if i == len(counts)-1 {
			// in the last bucket, estimate sum using the lower bound
			sum += float64(counts[i]) * bounds[i-1]
		} else {
			// in any other bucket, estimate sum using the bucket midpoint
			sum += float64(counts[i]) * (bounds[i] + bounds[i-1]) / 2
		}
	}

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
