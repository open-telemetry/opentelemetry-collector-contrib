// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package serialization // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/dynatraceexporter/internal/serialization"

import (
	dtMetric "github.com/dynatrace-oss/dynatrace-metric-utils-go/metric"
	"github.com/dynatrace-oss/dynatrace-metric-utils-go/metric/dimensions"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

func serializeHistogramPoint(name, prefix string, dims dimensions.NormalizedDimensionList, dp pmetric.HistogramDataPoint) (string, error) {
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

func serializeHistogram(logger *zap.Logger, prefix string, metric pmetric.Metric, defaultDimensions dimensions.NormalizedDimensionList, staticDimensions dimensions.NormalizedDimensionList, metricLines []string) []string {
	hist := metric.Histogram()

	if hist.AggregationTemporality() == pmetric.AggregationTemporalityCumulative {
		logger.Warn(
			"dropping cumulative histogram",
			zap.String("name", metric.Name()),
		)
		return metricLines
	}

	for i := 0; i < hist.DataPoints().Len(); i++ {
		dp := hist.DataPoints().At(i)

		line, err := serializeHistogramPoint(
			metric.Name(),
			prefix,
			makeCombinedDimensions(defaultDimensions, dp.Attributes(), staticDimensions),
			dp,
		)

		if err != nil {
			logger.Warn(
				"Error serializing histogram data point",
				zap.String("name", metric.Name()),
				zap.Error(err),
			)
		}

		if line != "" {
			metricLines = append(metricLines, line)
		}
	}
	return metricLines
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
