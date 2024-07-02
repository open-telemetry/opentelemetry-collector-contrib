package metrics

import (
	"context"
	"fmt"
	"math"

	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
)

type convertExponentialHistToExplicitHistArguments struct {
	ExplicitBounds []float64
}

func newconvertExponentialHistToExplicitHistFactory() ottl.Factory[ottlmetric.TransformContext] {
	return ottl.NewFactory("convert_exponential_hist_to_explicit_hist", &convertExponentialHistToExplicitHistArguments{}, createconvertExponentialHistToExplicitHistFunction)
}

func createconvertExponentialHistToExplicitHistFunction(_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[ottlmetric.TransformContext], error) {
	args, ok := oArgs.(*convertExponentialHistToExplicitHistArguments)

	if !ok {
		return nil, fmt.Errorf("convertExponentialHistToExplicitHistFactory args must be of type *convertExponentialHistToExplicitHistArguments")
	}

	return convertExponentialHistToExplicitHist(args.ExplicitBounds)
}

// convertExponentialHistToExplicitHist converts an exponential histogram to a bucketed histogram
func convertExponentialHistToExplicitHist(explicitBounds []float64) (ottl.ExprFunc[ottlmetric.TransformContext], error) {

	if len(explicitBounds) == 0 {
		return nil, fmt.Errorf("explicit bounds must cannot be empty: %v", explicitBounds)
	}

	return func(_ context.Context, tCtx ottlmetric.TransformContext) (any, error) {
		metric := tCtx.GetMetric()

		// only execute on exponential histograms
		if metric.Type() != pmetric.MetricTypeExponentialHistogram {
			return nil, nil
		}

		bucketedHist := pmetric.NewHistogram()
		dps := metric.ExponentialHistogram().DataPoints()
		bucketedHist.SetAggregationTemporality(metric.ExponentialHistogram().AggregationTemporality())

		// map over each exponential histogram data point and calculate the bucket counts
		for i := 0; i < dps.Len(); i++ {
			expDataPoint := dps.At(i)
			bucketCounts := calculateBucketCounts(expDataPoint, explicitBounds)
			bucketHistDatapoint := bucketedHist.DataPoints().AppendEmpty()
			bucketHistDatapoint.SetStartTimestamp(expDataPoint.StartTimestamp())
			bucketHistDatapoint.SetTimestamp(expDataPoint.Timestamp())
			bucketHistDatapoint.SetCount(expDataPoint.Count())
			bucketHistDatapoint.SetSum(expDataPoint.Sum())
			bucketHistDatapoint.SetMin(expDataPoint.Min())
			bucketHistDatapoint.SetMax(expDataPoint.Max())
			bucketHistDatapoint.ExplicitBounds().FromRaw(explicitBounds)
			bucketHistDatapoint.BucketCounts().FromRaw(bucketCounts)
			expDataPoint.Attributes().CopyTo(bucketHistDatapoint.Attributes())
		}

		// create new metric and override metric
		newMetric := pmetric.NewMetric()
		newMetric.SetName(metric.Name())
		newMetric.SetDescription(metric.Description())
		newMetric.SetUnit(metric.Unit())
		bucketedHist.CopyTo(newMetric.SetEmptyHistogram())
		newMetric.CopyTo(metric)

		return nil, nil
	}, nil
}

// calculateBucketCounts function calculates the bucket counts for a given exponential histogram data point.
// The algorithm is inspired by the logExponentialHistogramDataPoints function used to Print Exponential Histograms in Otel.
// found here: https://github.com/open-telemetry/opentelemetry-collector/blob/main/exporter/internal/otlptext/databuffer.go#L144-L201
//
// - factor is calculated as math.Ldexp(math.Ln2, -scale)
//
// - next we iterate the bucket counts and positions (pos) in the exponential histogram datapoint.
//
//   - the index is calculated by adding the exponential offset to the positive bucket position (pos)
//
//   - the factor is then used to calculate the upper bound of the bucket which is calculated as
//     upper = math.Exp((index+1) * factor)
//
//   - At this point we know that the upper bound represents the highest value that can be in this bucket, so we take the
//     upper bound and compare it to each of the explicit boundaries provided by the user until we find a boundary
//     that fits, that is, the first instance where upper bound <= explicit boundary.
func calculateBucketCounts(dp pmetric.ExponentialHistogramDataPoint, bounderies []float64) []uint64 {
	scale := int(dp.Scale())
	factor := math.Ldexp(math.Ln2, -scale)
	posB := dp.Positive().BucketCounts()
	bucketCounts := make([]uint64, len(bounderies)+1) // +1 for the overflow bucket

	for pos := 0; pos < posB.Len(); pos++ {
		index := dp.Positive().Offset() + int32(pos)
		upper := math.Exp(float64(index+1) * factor)
		count := posB.At(pos)

		for j, boundary := range bounderies {
			if upper <= boundary {
				bucketCounts[j] += count
				break
			}
			if j == len(bounderies)-1 {
				bucketCounts[j+1] += count // Overflow bucket
			}
		}
	}

	return bucketCounts
}
