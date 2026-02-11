// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewrite // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheusremotewrite"

import (
	"fmt"
	"math"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/value"
	writev2 "github.com/prometheus/prometheus/prompb/io/prometheus/write/v2"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/multierr"
)

func (c *prometheusConverterV2) addExponentialHistogramDataPoints(dataPoints pmetric.ExponentialHistogramDataPointSlice,
	resource pcommon.Resource, scope pcommon.InstrumentationScope, settings Settings, name string, metadata metadata,
) error {
	var errs error
	for x := 0; x < dataPoints.Len(); x++ {
		pt := dataPoints.At(x)

		histogram, err := exponentialToNativeHistogramV2(pt)
		if err != nil {
			errs = multierr.Append(errs, err)
			continue
		}

		lbls, err := createAttributes(resource, pt.Attributes(), scope, settings.ExternalLabels, nil, false, c.labelNamer, model.MetricNameLabel, name)
		if err != nil {
			errs = multierr.Append(errs, err)
			continue
		}

		ts := c.getOrCreateTimeSeries(lbls, metadata)
		ts.Histograms = append(ts.Histograms, histogram)

		// TODO handle exemplars
	}

	return errs
}

// exponentialToNativeHistogramV2 translates OTel Exponential Histogram data point
// to Prometheus Native Histogram.
func exponentialToNativeHistogramV2(p pmetric.ExponentialHistogramDataPoint) (writev2.Histogram, error) {
	scale := p.Scale()
	if scale < -4 {
		return writev2.Histogram{},
			fmt.Errorf("cannot convert exponential to native histogram."+
				" Scale must be >= -4, was %d", scale)
	}

	var scaleDown int32
	if scale > 8 {
		scaleDown = scale - 8
		scale = 8
	}

	pSpans, pDeltas := convertBucketsLayoutV2(p.Positive(), scaleDown)
	nSpans, nDeltas := convertBucketsLayoutV2(p.Negative(), scaleDown)

	h := writev2.Histogram{
		// The counter reset detection must be compatible with Prometheus to
		// safely set ResetHint to NO. This is not ensured currently.
		// Sending a sample that triggers counter reset but with ResetHint==NO
		// would lead to Prometheus panic as it does not double check the hint.
		// Thus we're explicitly saying UNKNOWN here, which is always safe.
		// TODO: using created time stamp should be accurate, but we
		// need to know here if it was used for the detection.
		// Ref: https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/28663#issuecomment-1810577303
		// Counter reset detection in Prometheus: https://github.com/prometheus/prometheus/blob/f997c72f294c0f18ca13fa06d51889af04135195/tsdb/chunkenc/histogram.go#L232
		ResetHint: writev2.Histogram_RESET_HINT_UNSPECIFIED,
		Schema:    scale,

		ZeroCount: &writev2.Histogram_ZeroCountInt{ZeroCountInt: p.ZeroCount()},
		// TODO use zero_threshold, if set, see
		// https://github.com/open-telemetry/opentelemetry-proto/pull/441
		ZeroThreshold: defaultZeroThreshold,

		PositiveSpans:  pSpans,
		PositiveDeltas: pDeltas,
		NegativeSpans:  nSpans,
		NegativeDeltas: nDeltas,

		Timestamp: convertTimeStamp(p.Timestamp()),
	}

	if p.Flags().NoRecordedValue() {
		h.Sum = math.Float64frombits(value.StaleNaN)
		h.Count = &writev2.Histogram_CountInt{CountInt: value.StaleNaN}
	} else {
		if p.HasSum() {
			h.Sum = p.Sum()
		}
		h.Count = &writev2.Histogram_CountInt{CountInt: p.Count()}
	}
	return h, nil
}

// convertBucketsLayoutV2 translates OTel Exponential Histogram dense buckets
// representation to Prometheus Native Histogram sparse bucket representation.
//
// The translation logic is taken from the client_golang `histogram.go#makeBuckets`
// function, see `makeBuckets` https://github.com/prometheus/client_golang/blob/main/prometheus/histogram.go
// The bucket indexes conversion was adjusted, since OTel exp. histogram bucket
// index 0 corresponds to the range (1, base] while Prometheus bucket index 0
// to the range (base 1].
//
// scaleDown is the factor by which the buckets are scaled down. In other words 2^scaleDown buckets will be merged into one.
func convertBucketsLayoutV2(buckets pmetric.ExponentialHistogramDataPointBuckets, scaleDown int32) ([]writev2.BucketSpan, []int64) {
	bucketCounts := buckets.BucketCounts()
	if bucketCounts.Len() == 0 {
		return nil, nil
	}

	var (
		spans     []writev2.BucketSpan
		deltas    []int64
		count     int64
		prevCount int64
	)

	appendDelta := func(count int64) {
		spans[len(spans)-1].Length++
		deltas = append(deltas, count-prevCount)
		prevCount = count
	}

	// Let the compiler figure out that this is const during this function by
	// moving it into a local variable.
	numBuckets := bucketCounts.Len()

	// The offset is scaled and adjusted by 1 as described above.
	bucketIdx := buckets.Offset()>>scaleDown + 1
	spans = append(spans, writev2.BucketSpan{
		Offset: bucketIdx,
		Length: 0,
	})

	for i := range numBuckets {
		// The offset is scaled and adjusted by 1 as described above.
		nextBucketIdx := (int32(i)+buckets.Offset())>>scaleDown + 1
		if bucketIdx == nextBucketIdx { // We have not collected enough buckets to merge yet.
			count += int64(bucketCounts.At(i))
			continue
		}
		if count == 0 {
			count = int64(bucketCounts.At(i))
			continue
		}

		gap := nextBucketIdx - bucketIdx - 1
		if gap > 2 {
			// We have to create a new span, because we have found a gap
			// of more than two buckets. The constant 2 is copied from the logic in
			// https://github.com/prometheus/client_golang/blob/27f0506d6ebbb117b6b697d0552ee5be2502c5f2/prometheus/histogram.go#L1296
			spans = append(spans, writev2.BucketSpan{
				Offset: gap,
				Length: 0,
			})
		} else {
			// We have found a small gap (or no gap at all).
			// Insert empty buckets as needed.
			for range gap {
				appendDelta(0)
			}
		}
		appendDelta(count)
		count = int64(bucketCounts.At(i))
		bucketIdx = nextBucketIdx
	}
	// Need to use the last item's index. The offset is scaled and adjusted by 1 as described above.
	gap := (int32(numBuckets)+buckets.Offset()-1)>>scaleDown + 1 - bucketIdx
	if gap > 2 {
		// We have to create a new span, because we have found a gap
		// of more than two buckets. The constant 2 is copied from the logic in
		// https://github.com/prometheus/client_golang/blob/27f0506d6ebbb117b6b697d0552ee5be2502c5f2/prometheus/histogram.go#L1296
		spans = append(spans, writev2.BucketSpan{
			Offset: gap,
			Length: 0,
		})
	} else {
		// We have found a small gap (or no gap at all).
		// Insert empty buckets as needed.
		for range gap {
			appendDelta(0)
		}
	}
	appendDelta(count)

	return spans, deltas
}
