// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/metrics"

import (
	"context"
	"errors"
	"fmt"
	"math"

	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
)

const percentileFuncName = "extract_percentile_metric"

type extractPercentileMetricArguments struct {
	Percentile float64
	Suffix     ottl.Optional[string]
}

func newExtractPercentileMetricFactory() ottl.Factory[*ottlmetric.TransformContext] {
	return ottl.NewFactory(percentileFuncName, &extractPercentileMetricArguments{}, createExtractPercentileMetricFunction)
}

func createExtractPercentileMetricFunction(_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[*ottlmetric.TransformContext], error) {
	args, ok := oArgs.(*extractPercentileMetricArguments)
	if !ok {
		return nil, errors.New("extractPercentileMetricFactory args must be of type *extractPercentileMetricArguments")
	}

	if args.Percentile < 0 || args.Percentile > 100 {
		return nil, fmt.Errorf("percentile must be between 0 and 100, got %f", args.Percentile)
	}

	return extractPercentileMetric(args.Percentile, args.Suffix)
}

func extractPercentileMetric(percentile float64, suffix ottl.Optional[string]) (ottl.ExprFunc[*ottlmetric.TransformContext], error) {
	var metricNameSuffix string
	if !suffix.IsEmpty() {
		metricNameSuffix = suffix.Get()
	} else {
		metricNameSuffix = fmt.Sprintf("_p%g", percentile)
	}

	return func(_ context.Context, tCtx *ottlmetric.TransformContext) (any, error) {
		metric := tCtx.GetMetric()

		if metric.Type() != pmetric.MetricTypeHistogram &&
			metric.Type() != pmetric.MetricTypeExponentialHistogram &&
			metric.Type() != pmetric.MetricTypeSummary {
			return nil, invalidMetricTypeError(percentileFuncName, metric)
		}

		percentileMetric := pmetric.NewMetric()
		percentileMetric.SetDescription(fmt.Sprintf("%s (p%g)", metric.Description(), percentile))
		percentileMetric.SetName(metric.Name() + metricNameSuffix)
		percentileMetric.SetUnit(metric.Unit())
		percentileMetric.SetEmptyGauge()
		gaugeDataPoints := percentileMetric.Gauge().DataPoints()

		switch metric.Type() {
		case pmetric.MetricTypeHistogram:
			if err := extractPercentileFromDataPoints(metric.Histogram().DataPoints(), percentile, gaugeDataPoints, calculateHistogramPercentile); err != nil {
				return nil, err
			}
		case pmetric.MetricTypeExponentialHistogram:
			if err := extractPercentileFromDataPoints(metric.ExponentialHistogram().DataPoints(), percentile, gaugeDataPoints, calculateExponentialHistogramPercentile); err != nil {
				return nil, err
			}
		case pmetric.MetricTypeSummary:
			if err := extractPercentileFromDataPoints(metric.Summary().DataPoints(), percentile, gaugeDataPoints, extractSummaryQuantile); err != nil {
				return nil, err
			}
		}

		if percentileMetric.Gauge().DataPoints().Len() > 0 {
			percentileMetric.MoveTo(tCtx.GetMetrics().AppendEmpty())
		}

		return nil, nil
	}, nil
}

func extractPercentileFromDataPoints[T dataPoint[T]](dataPoints dataPointSlice[T], percentile float64, destination pmetric.NumberDataPointSlice, calculateFunc func(T, float64) (float64, error)) error {
	for i := range dataPoints.Len() {
		dp := dataPoints.At(i)
		percentileValue, err := calculateFunc(dp, percentile)
		if err != nil {
			return err
		}
		addPercentileDataPoint(dp, percentileValue, destination)
	}
	return nil
}

func addPercentileDataPoint[T dataPoint[T]](sourceDP T, percentileValue float64, destination pmetric.NumberDataPointSlice) {
	newDp := destination.AppendEmpty()
	sourceDP.Attributes().CopyTo(newDp.Attributes())
	newDp.SetDoubleValue(percentileValue)
	newDp.SetStartTimestamp(sourceDP.StartTimestamp())
	newDp.SetTimestamp(sourceDP.Timestamp())
}

func calculateHistogramPercentile(dp pmetric.HistogramDataPoint, percentile float64) (float64, error) {
	if dp.Count() == 0 || dp.BucketCounts().Len() == 0 || dp.ExplicitBounds().Len() == 0 {
		return math.NaN(), nil
	}

	targetCount := uint64(math.Ceil(float64(dp.Count()) * (percentile / 100.0)))
	bucketCounts := dp.BucketCounts()
	explicitBounds := dp.ExplicitBounds()

	var cumulativeCount uint64
	for bucketIdx := range bucketCounts.Len() {
		previousCumulativeCount := cumulativeCount
		cumulativeCount += bucketCounts.At(bucketIdx)

		if cumulativeCount >= targetCount {
			var lowerBound, upperBound float64
			if bucketIdx == 0 {
				lowerBound = 0
				upperBound = explicitBounds.At(0)
			} else {
				lowerBound = explicitBounds.At(bucketIdx - 1)
				if bucketIdx < explicitBounds.Len() {
					upperBound = explicitBounds.At(bucketIdx)
				} else {
					// Last bucket's upper bound is +Inf unless Max is set.
					// Use Max for interpolation if available; otherwise return lowerBound.
					// https://opentelemetry.io/docs/specs/otel/metrics/data-model/#histogram-bucket-inclusivity
					if dp.HasMax() && dp.Max() > lowerBound {
						upperBound = dp.Max()
					} else {
						return lowerBound, nil
					}
				}
			}

			// Linear interpolation within bucket. Note: this is a simplification and assumes uniform distribution within the bucket.
			bucketCount := bucketCounts.At(bucketIdx)
			ratio := float64(targetCount-previousCumulativeCount) / float64(bucketCount)
			return linearInterpolation(lowerBound, upperBound, ratio), nil
		}
	}
	return 0, fmt.Errorf(
		"percentile p%.2f not found in histogram buckets:"+
			" targetCount=%d, totalCount=%d, total cumulative count=%d (malformed data: cumulative count doesn't reach target)",
		percentile,
		targetCount,
		dp.Count(),
		cumulativeCount,
	)
}

func calculateExponentialHistogramPercentile(dp pmetric.ExponentialHistogramDataPoint, percentile float64) (float64, error) {
	if dp.Count() == 0 || (dp.Negative().BucketCounts().Len() == 0 && dp.Positive().BucketCounts().Len() == 0 && dp.ZeroCount() == 0) {
		return math.NaN(), nil
	}

	targetCount := uint64(math.Ceil(float64(dp.Count()) * (percentile / 100.0)))
	scale := int(dp.Scale())

	negativeBuckets := dp.Negative()
	zeroCount := dp.ZeroCount()
	positiveBuckets := dp.Positive()

	var negativeTotalCount uint64
	for i := range negativeBuckets.BucketCounts().Len() {
		previousCumulativeCount := negativeTotalCount
		negativeTotalCount += negativeBuckets.BucketCounts().At(i)
		if negativeTotalCount >= targetCount {
			return calculateFromNegativeBuckets(negativeBuckets, targetCount, previousCumulativeCount, i, scale)
		}
	}

	cumulativeAfterZero := negativeTotalCount + zeroCount
	if cumulativeAfterZero >= targetCount {
		return calculateFromZeroBucket(dp, negativeBuckets.BucketCounts().Len() != 0, positiveBuckets.BucketCounts().Len() != 0, targetCount, negativeTotalCount, zeroCount)
	}

	return calculateFromPositiveBuckets(positiveBuckets, targetCount, cumulativeAfterZero, scale, percentile)
}

func calculateFromZeroBucket(dp pmetric.ExponentialHistogramDataPoint, negativeBuckets, positiveBuckets bool, targetCount, negativeTotalCount, zeroCount uint64) (float64, error) {
	zeroBucketLower := -dp.ZeroThreshold()
	zeroBucketUpper := dp.ZeroThreshold()

	// If only positive buckets exist, assume zero bucket lower bound is 0
	if !negativeBuckets && positiveBuckets {
		zeroBucketLower = 0
	}

	// If only negative buckets exist, assume zero bucket upper bound is 0
	if !positiveBuckets && negativeBuckets {
		zeroBucketUpper = 0
	}

	ratio := float64(targetCount-negativeTotalCount) / float64(zeroCount)
	return linearInterpolation(zeroBucketLower, zeroBucketUpper, ratio), nil
}

func calculateFromNegativeBuckets(buckets pmetric.ExponentialHistogramDataPointBuckets, targetCount, previousCumulativeCount uint64, bucketIdx, scale int) (float64, error) {
	bucketCount := buckets.BucketCounts().At(bucketIdx)
	bucketIndex := int(buckets.Offset()) + bucketIdx

	upperBound := calculateExponentialBucketBound(bucketIndex, scale)
	lowerBound := calculateExponentialBucketBound(bucketIndex+1, scale)

	ratio := (float64(targetCount) - float64(previousCumulativeCount)) / float64(bucketCount)
	// For negative buckets, invert ratio direction since we move from less negative to more negative
	return -logarithmicInterpolation(upperBound, lowerBound, 1-ratio), nil
}

func calculateFromPositiveBuckets(buckets pmetric.ExponentialHistogramDataPointBuckets, targetCount, cumulativeBefore uint64, scale int, percentile float64) (float64, error) {
	bucketCounts := buckets.BucketCounts()

	cumulativeCount := cumulativeBefore
	for i := range bucketCounts.Len() {
		previousCumulativeCount := cumulativeCount
		bucketCount := bucketCounts.At(i)
		cumulativeCount += bucketCount

		if cumulativeCount >= targetCount {
			bucketIndex := int(buckets.Offset()) + i
			lowerBound := calculateExponentialBucketBound(bucketIndex, scale)
			upperBound := calculateExponentialBucketBound(bucketIndex+1, scale)

			ratio := float64(targetCount-previousCumulativeCount) / float64(bucketCount)

			return logarithmicInterpolation(lowerBound, upperBound, ratio), nil
		}
	}

	return 0, fmt.Errorf("percentile p%.2f not found in positive buckets for exponential histogram:"+
		" targetCount=%d, total count=%d (malformed data: cumulative count doesn't reach target count corresponding to percentile)",
		percentile,
		targetCount,
		cumulativeCount,
	)
}

// linearInterpolation performs linear interpolation between two bounds.
// Assumes uniform distribution of observations within the bucket.
func linearInterpolation(lowerBound, upperBound, ratio float64) float64 {
	return lowerBound + ratio*(upperBound-lowerBound)
}

// logarithmicInterpolation performs logarithmic interpolation between two bounds,
// appropriate for exponential histograms.
func logarithmicInterpolation(lowerBound, upperBound, ratio float64) float64 {
	logLower := math.Log2(lowerBound)
	logUpper := math.Log2(upperBound)
	return math.Exp2(logLower + (logUpper-logLower)*ratio)
}

// calculateExponentialBucketBound calculates the lower bound of an exponential histogram bucket
// using the formulas from the OpenTelemetry specification.
// References: https://opentelemetry.io/docs/specs/otel/metrics/data-model/#negative-scale-extract-and-shift-the-exponent
// https://opentelemetry.io/docs/specs/otel/metrics/data-model/#all-scales-use-the-logarithm-function
func calculateExponentialBucketBound(index int, scale int) float64 {
	if scale <= 0 {
		return math.Ldexp(1, index<<-scale)
	} else {
		inverseFactor := math.Ldexp(math.Ln2, -scale)
		return math.Exp(float64(index) * inverseFactor)
	}
}

func extractSummaryQuantile(dp pmetric.SummaryDataPoint, percentile float64) (float64, error) {
	quantileValues := dp.QuantileValues()
	targetQuantile := percentile / 100.0

	for i := range quantileValues.Len() {
		qv := quantileValues.At(i)
		if qv.Quantile() == targetQuantile {
			return qv.Value(), nil
		}
	}

	availableQuantiles := make([]float64, quantileValues.Len())
	for i := range quantileValues.Len() {
		availableQuantiles[i] = quantileValues.At(i).Quantile() * 100.0
	}

	return 0, fmt.Errorf("percentile p%g not found in summary metric, available quantiles: %v", percentile, availableQuantiles)
}
