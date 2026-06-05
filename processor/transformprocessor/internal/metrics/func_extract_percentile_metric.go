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

// errSkipDataPoint is a sentinel error indicating that a data point should be skipped
// (e.g., because there are no buckets to interpolate from).
var errSkipDataPoint = errors.New("skipping data point")

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

	if args.Percentile <= 0 || args.Percentile >= 100 {
		return nil, fmt.Errorf("percentile must be greater than 0 and less than 100, got %f", args.Percentile)
	}

	return extractPercentileMetric(args.Percentile, args.Suffix)
}

func extractPercentileMetric(percentile float64, suffix ottl.Optional[string]) (ottl.ExprFunc[*ottlmetric.TransformContext], error) {
	var metricNameSuffix string
	if !suffix.IsEmpty() && suffix.Get() != "" {
		metricNameSuffix = suffix.Get()
	} else {
		metricNameSuffix = fmt.Sprintf("_p%g", percentile)
	}

	return func(_ context.Context, tCtx *ottlmetric.TransformContext) (any, error) {
		metric := tCtx.GetMetric()

		switch metric.Type() {
		case pmetric.MetricTypeHistogram:
			if metric.Histogram().DataPoints().Len() == 0 {
				return nil, nil
			}
		case pmetric.MetricTypeExponentialHistogram:
			if metric.ExponentialHistogram().DataPoints().Len() == 0 {
				return nil, nil
			}
		default:
			return nil, nil
		}

		percentileMetric := pmetric.NewMetric()
		if metric.Description() != "" {
			percentileMetric.SetDescription(fmt.Sprintf("%s (p%g)", metric.Description(), percentile))
		} else {
			percentileMetric.SetDescription(fmt.Sprintf("p%g", percentile))
		}
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
		default:
			return nil, nil
		}

		if percentileMetric.Gauge().DataPoints().Len() > 0 {
			percentileMetric.MoveTo(tCtx.GetMetrics().AppendEmpty())
		}

		return nil, nil
	}, nil
}

func extractPercentileFromDataPoints[T dataPoint](dataPoints dataPointSlice[T], percentile float64, destination pmetric.NumberDataPointSlice, calculateFunc func(T, float64) (float64, error)) error {
	for i := range dataPoints.Len() {
		dp := dataPoints.At(i)
		percentileValue, err := calculateFunc(dp, percentile)
		if errors.Is(err, errSkipDataPoint) {
			continue
		}
		if err != nil {
			return err
		}
		addPercentileDataPoint(dp, percentileValue, destination)
	}
	return nil
}

func addPercentileDataPoint[T dataPoint](sourceDP T, percentileValue float64, destination pmetric.NumberDataPointSlice) {
	newDp := destination.AppendEmpty()
	sourceDP.Attributes().CopyTo(newDp.Attributes())
	newDp.SetDoubleValue(percentileValue)
	newDp.SetStartTimestamp(sourceDP.StartTimestamp())
	newDp.SetTimestamp(sourceDP.Timestamp())
}

func calculateHistogramPercentile(dp pmetric.HistogramDataPoint, percentile float64) (float64, error) {
	if dp.Count() == 0 || dp.BucketCounts().Len() == 0 {
		return 0, errSkipDataPoint
	}

	targetCount := uint64(math.Ceil(float64(dp.Count()) * (percentile / 100.0)))
	bucketCounts := dp.BucketCounts()
	explicitBounds := dp.ExplicitBounds()
	if bucketCounts.Len() != explicitBounds.Len()+1 {
		return 0, fmt.Errorf("calculating histogram percentile: malformed data: bucketCounts length (%d) != explicitBounds length (%d) + 1",
			bucketCounts.Len(), explicitBounds.Len())
	}

	// Single-bucket histogram with no explicit bounds: spans (-Inf, +Inf).
	// Both Min and Max are required for meaningful interpolation.
	if bucketCounts.Len() == 1 && explicitBounds.Len() == 0 {
		if !dp.HasMin() || !dp.HasMax() {
			return 0, errSkipDataPoint
		}
		ratio := float64(targetCount) / float64(bucketCounts.At(0))
		return linearInterpolation(dp.Min(), dp.Max(), ratio)
	}

	var cumulativeCount uint64
	for bucketIdx := range bucketCounts.Len() {
		previousCumulativeCount := cumulativeCount
		cumulativeCount += bucketCounts.At(bucketIdx)

		if cumulativeCount >= targetCount {
			var lowerBound, upperBound float64
			if bucketIdx == 0 {
				upperBound = explicitBounds.At(0)
				lowerBound = 0
				if dp.HasMin() && dp.Min() < upperBound {
					lowerBound = dp.Min()
				} else if 0 > upperBound {
					// If 0 > upperBound and no valid Min is set,
					// return the upperBound as it's the minimum value we have, and we can't interpolate with -Inf.
					return upperBound, nil
				}
			} else {
				lowerBound = explicitBounds.At(bucketIdx - 1)
				if bucketIdx < explicitBounds.Len() {
					upperBound = explicitBounds.At(bucketIdx)
				} else {
					// Last bucket's upper bound is +Inf unless Max is set.
					// Use Max for interpolation if available; otherwise return lowerBound.
					// https://opentelemetry.io/docs/specs/otel/metrics/data-model/#histogram-bucket-inclusivity
					if !dp.HasMax() || dp.Max() <= lowerBound {
						return lowerBound, nil
					}
					upperBound = dp.Max()
				}
			}

			// Linear interpolation within bucket. Note: this is a simplification and assumes uniform distribution within the bucket.
			bucketCount := bucketCounts.At(bucketIdx)
			ratio := float64(targetCount-previousCumulativeCount) / float64(bucketCount)
			return linearInterpolation(lowerBound, upperBound, ratio)
		}
	}
	return 0, fmt.Errorf("calculating histogram percentile: malformed data: cumulative count doesn't reach target count corresponding to percentile. targetCount=%d, total count=%d",
		targetCount,
		cumulativeCount,
	)
}

func calculateExponentialHistogramPercentile(dp pmetric.ExponentialHistogramDataPoint, percentile float64) (float64, error) {
	if dp.Count() == 0 || (dp.Negative().BucketCounts().Len() == 0 && dp.Positive().BucketCounts().Len() == 0 && dp.ZeroCount() == 0) {
		return 0, errSkipDataPoint
	}

	targetCount := uint64(math.Ceil(float64(dp.Count()) * (percentile / 100.0)))
	scale := int(dp.Scale())

	negativeBuckets := dp.Negative()
	zeroCount := dp.ZeroCount()
	positiveBuckets := dp.Positive()

	var negativeTotalCount uint64
	for i := negativeBuckets.BucketCounts().Len() - 1; i >= 0; i-- {
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

	return calculateFromPositiveBuckets(positiveBuckets, targetCount, cumulativeAfterZero, scale)
}

func calculateFromZeroBucket(dp pmetric.ExponentialHistogramDataPoint, negativeBuckets, positiveBuckets bool, targetCount, negativeTotalCount, zeroCount uint64) (float64, error) {
	zeroBucketLower := -dp.ZeroThreshold()
	zeroBucketUpper := dp.ZeroThreshold()

	if !negativeBuckets && positiveBuckets {
		// If only positive buckets exist, assume zero bucket lower bound is 0
		zeroBucketLower = 0
	} else if !positiveBuckets && negativeBuckets {
		// If only negative buckets exist, assume zero bucket upper bound is 0
		zeroBucketUpper = 0
	}

	ratio := float64(targetCount-negativeTotalCount) / float64(zeroCount)
	return linearInterpolation(zeroBucketLower, zeroBucketUpper, ratio)
}

func calculateFromNegativeBuckets(buckets pmetric.ExponentialHistogramDataPointBuckets, targetCount, previousCumulativeCount uint64, bucketIdx, scale int) (float64, error) {
	bucketCount := buckets.BucketCounts().At(bucketIdx)
	histogramIndex := int(buckets.Offset()) + bucketIdx
	absLowerBound := calculateExponentialBucketBound(histogramIndex, scale)
	absUpperBound := calculateExponentialBucketBound(histogramIndex+1, scale)

	if absLowerBound >= absUpperBound {
		return 0, fmt.Errorf(
			"calculating exponential histogram percentile: malformed negative bucket bounds: absLowerBound (%.6f) >= absUpperBound (%.6f) at histogramIndex=%d, scale=%d",
			absLowerBound, absUpperBound, histogramIndex, scale,
		)
	}

	// ratio = 0 -> most-negative end of bucket (−absUpperBound)
	// ratio = 1 -> least-negative end of bucket (−absLowerBound)
	// Interpolating absUpper -> absLower and negating gives the correct direction.
	ratio := float64(targetCount-previousCumulativeCount) / float64(bucketCount)
	absValue, err := logarithmicInterpolation(absLowerBound, absUpperBound, 1-ratio)
	if err != nil {
		return 0, err
	}
	return -absValue, nil
}

func calculateFromPositiveBuckets(buckets pmetric.ExponentialHistogramDataPointBuckets, targetCount, cumulativeBefore uint64, scale int) (float64, error) {
	bucketCounts := buckets.BucketCounts()

	cumulativeCount := cumulativeBefore
	for i := range bucketCounts.Len() {
		previousCumulativeCount := cumulativeCount
		bucketCount := bucketCounts.At(i)
		cumulativeCount += bucketCount

		if cumulativeCount >= targetCount {
			histogramIndex := int(buckets.Offset()) + i
			lowerBound := calculateExponentialBucketBound(histogramIndex, scale)
			upperBound := calculateExponentialBucketBound(histogramIndex+1, scale)

			ratio := float64(targetCount-previousCumulativeCount) / float64(bucketCount)
			return logarithmicInterpolation(lowerBound, upperBound, ratio)
		}
	}

	return 0, fmt.Errorf("calculating exponential histogram percentile: malformed data: cumulative count doesn't reach target count corresponding to percentile. targetCount=%d, total count=%d",
		targetCount,
		cumulativeCount,
	)
}

// linearInterpolation performs linear interpolation between two bounds.
// Assumes uniform distribution of observations within the bucket.
func linearInterpolation(lowerBound, upperBound, ratio float64) (float64, error) {
	if ratio < 0 || ratio > 1 {
		return 0, fmt.Errorf("linear interpolation: invalid ratio %.6f, must be between 0 and 1", ratio)
	}
	return lowerBound + ratio*(upperBound-lowerBound), nil
}

// logarithmicInterpolation performs logarithmic interpolation between two bounds,
// appropriate for exponential histograms.
func logarithmicInterpolation(lowerBound, upperBound, ratio float64) (float64, error) {
	if ratio < 0 || ratio > 1 {
		return 0, fmt.Errorf("logarithmic interpolation: invalid ratio %.6f, must be between 0 and 1", ratio)
	}
	if lowerBound <= 0 || upperBound <= 0 {
		return 0, fmt.Errorf("logarithmic interpolation: bounds must be positive, got lowerBound=%.6f, upperBound=%.6f", lowerBound, upperBound)
	}
	logLower := math.Log2(lowerBound)
	logUpper := math.Log2(upperBound)
	return math.Exp2(logLower + (logUpper-logLower)*ratio), nil
}

// calculateExponentialBucketBound calculates the lower bound of an exponential histogram bucket
// using the formulas from the OpenTelemetry specification.
// References: https://opentelemetry.io/docs/specs/otel/metrics/data-model/#negative-scale-extract-and-shift-the-exponent
// https://opentelemetry.io/docs/specs/otel/metrics/data-model/#all-scales-use-the-logarithm-function
func calculateExponentialBucketBound(index, scale int) float64 {
	if scale <= 0 {
		return math.Ldexp(1, index<<-scale)
	}
	inverseFactor := math.Ldexp(math.Ln2, -scale)
	return math.Exp(float64(index) * inverseFactor)
}
