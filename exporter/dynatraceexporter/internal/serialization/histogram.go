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

	min, max := estimateHistMinMax(dp)

	dm, err := dtMetric.NewMetric(
		name,
		dtMetric.WithPrefix(prefix),
		dtMetric.WithDimensions(dims),
		dtMetric.WithTimestamp(dp.Timestamp().AsTime()),
		dtMetric.WithFloatSummaryValue(
			min,
			max,
			dp.Sum(),
			int64(dp.Count()),
		),
	)

	if err != nil {
		return "", err
	}

	return dm.Serialize()
}

// estimateHistMinMax returns the estimated minimum and maximum value in the histogram by using the min and max non-empty buckets.
func estimateHistMinMax(dp pmetric.HistogramDataPoint) (float64, float64) {
	bounds := dp.MExplicitBounds()
	counts := dp.MBucketCounts()

	// shortcut in the case both max and min are provided
	if dp.HasMin() && dp.HasMax() {
		return dp.Min(), dp.Max()
	}

	// Because we do not know the actual min and max, we estimate them based on the min and max non-empty bucket
	minIdx, maxIdx := -1, -1
	for y := 0; y < len(counts); y++ {
		if counts[y] > 0 {
			if minIdx == -1 {
				minIdx = y
			}
			maxIdx = y
		}
	}

	if minIdx == -1 || maxIdx == -1 {
		return 0, 0
	}

	var min, max float64

	if dp.HasMin() {
		min = dp.Min()
	} else {
		// Use lower bound for min unless it is the first bucket which has no lower bound, then use upper
		if minIdx == 0 {
			min = bounds[minIdx]
		} else {
			min = bounds[minIdx-1]
		}
	}

	if dp.HasMax() {
		max = dp.Max()
	} else {
		// Use upper bound for max unless it is the last bucket which has no upper bound, then use lower
		if maxIdx == len(counts)-1 {
			max = bounds[maxIdx-1]
		} else {
			max = bounds[maxIdx]
		}
	}

	// Set min to average when higher than average. This can happen when most values are lower than first boundary (falling in first bucket).
	// Set max to average when lower than average. This can happen when most values are higher than last boundary (falling in last bucket).
	avg := dp.Sum() / float64(dp.Count())
	if min > avg {
		min = avg
	}
	if max < avg {
		max = avg
	}

	return min, max
}
