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

		// expHist := metric.ExponentialHistogram()
		bucketedHist := pmetric.NewHistogram()
		dps := metric.ExponentialHistogram().DataPoints()
		bucketedHist.SetAggregationTemporality(metric.ExponentialHistogram().AggregationTemporality())

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

// calculateBucketCounts calculates the bucket counts for a given exponential histogram data point
// the algorithm is based on the OpenTelemetry Collector implementation
//
//   - base is calculated as 2^-scale
//
//   - the base is then used to calculate the upper bound of the bucket
//     which is calculated as base^(index+1)
//
// - the index is calculated, by adding the offset to the positive bucket index
//
// - the upper limit is the exponential of the index+1 times the base
//
// - upper bound is used to determine which of the explicit bounds the bucket count falls into
func calculateBucketCounts(dp pmetric.ExponentialHistogramDataPoint, bounderies []float64) []uint64 {
	scale := int(dp.Scale())
	base := math.Ldexp(math.Ln2, -scale)

	// negB := dp.Negative().BucketCounts()
	posB := dp.Positive().BucketCounts()
	bucketCounts := make([]uint64, len(bounderies)+1) // +1 for the overflow bucket

	for pos := 0; pos < posB.Len(); pos++ {
		index := dp.Positive().Offset() + int32(pos)
		upper := math.Exp(float64(index+1) * base)
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
