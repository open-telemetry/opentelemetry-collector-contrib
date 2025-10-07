// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translator // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogreceiver/internal/translator"

import (
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"time"

	"github.com/DataDog/agent-payload/v5/gogen"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/identity"
)

const (
	// The relativeAccuracy (also called epsilon or eps) comes from DDSketch's logarithmic mapping, which is used for sketches
	// in the Datadog agent. The Datadog agent uses the default value from opentelemetry-go-mapping configuration
	// See:
	// https://github.com/DataDog/datadog-agent/blob/fcb59435e45053bcb53a1eec482104290f1dd166/pkg/util/quantile/config.go#L15
	relativeAccuracy = 1.0 / 128

	// The gamma value comes from the default values of the epsilon/relative accuracy from opentelemetry-go-mapping. This value is used for
	// finding the lower boundary of the bucket at a specific index
	// See:
	// https://github.com/DataDog/datadog-agent/blob/fcb59435e45053bcb53a1eec482104290f1dd166/pkg/util/quantile/config.go#L138
	gamma = 1 + 2*relativeAccuracy

	// Since the default bucket factor for Sketches (gamma value) is 1.015625, this corresponds to a scale between 5 (2^2^-5=1.0219)
	// and 6 (2^2^-6=1.01088928605). However, the lower resolution of 5 will produce larger buckets which allows for easier mapping
	scale = 5

	// The agentSketchOffset value comes from the following calculation:
	// min = 1e-9
	// emin = math.Floor((math.Log(min)/math.Log1p(2*relativeAccuracy))
	// offset = -emin + 1
	// The resulting value is 1338.
	// See: https://github.com/DataDog/datadog-agent/blob/fcb59435e45053bcb53a1eec482104290f1dd166/pkg/util/quantile/config.go#L154
	// (Note: in Datadog's code, it is referred to as 'bias')
	agentSketchOffset int32 = 1338

	// The max limit for the index of a sketch bucket
	// See https://github.com/DataDog/datadog-agent/blob/fcb59435e45053bcb53a1eec482104290f1dd166/pkg/util/quantile/ddsketch.go#L21
	// and https://github.com/DataDog/datadog-agent/blob/fcb59435e45053bcb53a1eec482104290f1dd166/pkg/util/quantile/ddsketch.go#L138
	maxIndex = math.MaxInt16
)

// Unmarshal the sketch payload, which contains the underlying Dogsketch structure used for the translation
func (*MetricsTranslator) HandleSketchesPayload(req *http.Request) (sp []gogen.SketchPayload_Sketch, err error) {
	buf := GetBuffer()
	defer PutBuffer(buf)
	if _, err := io.Copy(buf, req.Body); err != nil {
		return sp, err
	}

	pl := new(gogen.SketchPayload)
	if err := pl.Unmarshal(buf.Bytes()); err != nil {
		return sp, err
	}

	return pl.GetSketches(), nil
}

func (mt *MetricsTranslator) TranslateSketches(sketches []gogen.SketchPayload_Sketch) pmetric.Metrics {
	bt := newBatcher()
	bt.Metrics = pmetric.NewMetrics()

	for i := range sketches {
		sketch := &sketches[i]
		dimensions := parseSeriesProperties(sketch.Metric, "sketch", sketch.Tags, sketch.Host, mt.buildInfo.Version, mt.stringPool)
		metric, metricID := bt.Lookup(dimensions)
		metric.ExponentialHistogram().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
		dps := metric.ExponentialHistogram().DataPoints()

		dps.EnsureCapacity(len(sketch.Dogsketches))

		// The dogsketches field of the payload contains the sketch data
		for i := range sketch.Dogsketches {
			dp := dps.AppendEmpty()

			err := sketchToDatapoint(sketch.Dogsketches[i], dp, dimensions.dpAttrs)
			if err != nil {
				// If a sketch is invalid, remove this datapoint
				metric.ExponentialHistogram().DataPoints().RemoveIf(func(dp pmetric.ExponentialHistogramDataPoint) bool {
					if dp.Positive().BucketCounts().Len() == 0 && dp.Negative().BucketCounts().Len() == 0 {
						return true
					}
					return false
				})
				continue
			}
			stream := identity.OfStream(metricID, dp)
			if ts, ok := mt.streamHasTimestamp(stream); ok {
				dp.SetStartTimestamp(ts)
			}
			mt.updateLastTsForStream(stream, dp.Timestamp())
		}
	}

	return bt.Metrics
}

func sketchToDatapoint(sketch gogen.SketchPayload_Sketch_Dogsketch, dp pmetric.ExponentialHistogramDataPoint, attributes pcommon.Map) error {
	dp.SetTimestamp(pcommon.Timestamp(sketch.Ts * time.Second.Nanoseconds())) // OTel uses nanoseconds, while Datadog uses seconds

	dp.SetCount(uint64(sketch.Cnt))
	dp.SetSum(sketch.Sum)
	dp.SetMin(sketch.Min)
	dp.SetMax(sketch.Max)
	dp.SetScale(scale)
	dp.SetZeroThreshold(math.Exp(float64(1-agentSketchOffset) / (1 / math.Log(gamma)))) // See https://github.com/DataDog/sketches-go/blob/7546f8f95179bb41d334d35faa281bfe97812a86/ddsketch/mapping/logarithmic_mapping.go#L48

	attributes.CopyTo(dp.Attributes())

	negativeBuckets, positiveBuckets, zeroCount, err := mapSketchBucketsToHistogramBuckets(sketch.K, sketch.N)
	if err != nil {
		return err
	}

	dp.SetZeroCount(zeroCount)

	convertBucketLayout(positiveBuckets, dp.Positive())
	convertBucketLayout(negativeBuckets, dp.Negative())

	return nil
}

// mapSketchBucketsToHistogramBuckets attempts to map the counts in each Sketch bucket to the closest equivalent Exponential Histogram
// bucket(s). It works by first calculating an Exponential Histogram key that corresponds most closely with the Sketch key (using the lower
// bound of the sketch bucket the key corresponds to), calculates differences in the range of the Sketch bucket and exponential histogram bucket,
// and distributes the count to the corresponding bucket, and the bucket(s) after it, based on the proportion of overlap between the
// exponential histogram buckets and the Sketch bucket. Note that the Sketch buckets are not separated into positive and negative buckets, but exponential
// histograms store positive and negative buckets separately. Negative buckets in exponential histograms are mapped in the same way as positive buckets.
// Note that negative indices in exponential histograms do not necessarily correspond to negative values; they correspond with values between 0 and 1,
// on either the negative or positive side
func mapSketchBucketsToHistogramBuckets(sketchKeys []int32, sketchCounts []uint32) (map[int]uint64, map[int]uint64, uint64, error) {
	var zeroCount uint64

	positiveBuckets := make(map[int]uint64)
	negativeBuckets := make(map[int]uint64)

	// The data format for the sketch received from the sketch payload does not have separate positive and negative buckets,
	// and instead just uses a single list of sketch keys that are in order by increasing bucket index, starting with negative indices,
	// which correspond to negative buckets
	for i := range sketchKeys {
		if sketchKeys[i] == 0 { // A sketch key of 0 corresponds to the zero bucket
			zeroCount += uint64(sketchCounts[i])
			continue
		}
		if sketchKeys[i] >= maxIndex {
			// This should not happen, as sketches that contain bucket(s) with an index greater than the max
			// limit should have already been discarded. However, if there happens to be an index > maxIndex,
			// it can cause an infinite loop within the below inner for loop on some operating systems. Therefore,
			// throw an error for sketches that have an index above the max limit
			return nil, nil, 0, fmt.Errorf("Sketch contains bucket index %d which exceeds maximum supported index value %d", sketchKeys[i], maxIndex)
		}

		// The approach here is to use the Datadog sketch index's lower bucket boundary to find the
		// OTel exponential histogram bucket that with the closest range to the sketch bucket. Then,
		// the buckets before and after that bucket are also checked for overlap with the sketch bucket.
		// A count proportional to the intersection of the sketch bucket with the OTel bucket(s) is then
		// added to the OTel bucket(s). After looping through all possible buckets that are within the Sketch
		// bucket range, the bucket with the highest proportion of overlap is given the remaining count
		sketchLowerBound, sketchUpperBound := getSketchBounds(sketchKeys[i])
		sketchBucketSize := sketchUpperBound - sketchLowerBound
		histogramKey := sketchLowerBoundToHistogramIndex(sketchLowerBound)
		highestCountProportion := 0.0
		highestCountIdx := 0
		targetBucketCount := uint64(sketchCounts[i])
		var currentAssignedCount uint64

		// TODO: look into better algorithms for applying fractional counts
		for outIndex := histogramKey; histogramLowerBound(outIndex) < sketchUpperBound; outIndex++ {
			histogramLowerBound, histogramUpperBound := getHistogramBounds(outIndex)
			lowerIntersection := math.Max(histogramLowerBound, sketchLowerBound)
			higherIntersection := math.Min(histogramUpperBound, sketchUpperBound)

			intersectionSize := higherIntersection - lowerIntersection
			proportion := intersectionSize / sketchBucketSize
			if proportion <= 0 {
				continue // In this case, the bucket does not overlap with the sketch bucket, so continue to the next bucket
			}
			if proportion > highestCountProportion {
				highestCountProportion = proportion
				highestCountIdx = outIndex
			}
			// OTel exponential histograms only support integer bucket counts, so rounding needs to be done here
			roundedCount := uint64(proportion * float64(sketchCounts[i]))
			if sketchKeys[i] < 0 {
				negativeBuckets[outIndex] += roundedCount
			} else {
				positiveBuckets[outIndex] += roundedCount
			}
			currentAssignedCount += roundedCount
		}
		// Add the difference between the original sketch bucket's count and the total count that has been
		// added to the matching OTel bucket(s) thus far to the bucket that had the highest proportion of
		// overlap between the original sketch bucket and the corresponding exponential histogram buckets
		if highestCountProportion > 0 {
			additionalCount := targetBucketCount - currentAssignedCount
			if sketchKeys[i] < 0 {
				negativeBuckets[highestCountIdx] += additionalCount
			} else {
				positiveBuckets[highestCountIdx] += additionalCount
			}
		}
	}

	return negativeBuckets, positiveBuckets, zeroCount, nil
}

// convertBucketLayout populates the count for positive or negative buckets in the resulting OTel
// exponential histogram structure. The bucket layout is dense and consists of an offset, which is the
// index of the first populated bucket, and a list of counts, which correspond to the counts at the offset
// bucket's index, and the counts of each bucket after. Unpopulated/empty buckets must be represented with
// a count of 0. After assigning bucket counts, it sets the offset for the bucket layout
func convertBucketLayout(inputBuckets map[int]uint64, outputBuckets pmetric.ExponentialHistogramDataPointBuckets) {
	if len(inputBuckets) == 0 {
		return
	}
	bucketIdxs := make([]int, 0, len(inputBuckets))
	for k := range inputBuckets {
		bucketIdxs = append(bucketIdxs, k)
	}
	sort.Ints(bucketIdxs)

	bucketsSize := bucketIdxs[len(bucketIdxs)-1] - bucketIdxs[0] + 1 // find total number of buckets needed
	outputBuckets.BucketCounts().EnsureCapacity(bucketsSize)
	outputBuckets.BucketCounts().Append(make([]uint64, bucketsSize)...)

	offset := bucketIdxs[0]
	outputBuckets.SetOffset(int32(offset))

	for _, idx := range bucketIdxs {
		delta := idx - offset
		outputBuckets.BucketCounts().SetAt(delta, inputBuckets[idx])
	}
}

// getSketchBounds calculates the lower and upper bounds of a sketch bucket based on the index of the bucket.
// This is based on sketch buckets placing values in bucket so that γ^k <= v < γ^(k+1)
// See https://github.com/DataDog/datadog-agent/blob/0ada7a97fed6727838a6f4d9c87123d2aafde735/pkg/quantile/config.go#L83
// and https://github.com/DataDog/sketches-go/blob/8a1961cf57f80fbbe26e7283464fcc01ebf17d5c/ddsketch/ddsketch.go#L468
func getSketchBounds(index int32) (float64, float64) {
	if index < 0 {
		index = -index
	}
	return sketchLowerBound(index), sketchLowerBound(index + 1)
}

// sketchLowerBound calculates the lower bound of a sketch bucket based on the index of the bucket.
// It uses the index offset and multiplier (represented by (1 / math.Log(gamma))). The logic behind this
// is based on the DD agent using logarithmic mapping for definition DD agent sketches
// See:
// https://github.com/DataDog/datadog-agent/blob/fcb59435e45053bcb53a1eec482104290f1dd166/pkg/util/quantile/config.go#L54
// https://github.com/DataDog/sketches-go/blob/8a1961cf57f80fbbe26e7283464fcc01ebf17d5c/ddsketch/mapping/logarithmic_mapping.go#L39
func sketchLowerBound(index int32) float64 {
	if index < 0 {
		index = -index
	}
	return math.Exp((float64(index-agentSketchOffset) / (1 / math.Log(gamma))))
}

// getHistogramBounds returns the lower and upper boundaries of the histogram bucket that
// corresponds to the specified bucket index
func getHistogramBounds(histIndex int) (float64, float64) {
	return histogramLowerBound(histIndex), histogramLowerBound(histIndex + 1)
}

// This equation for finding the lower bound of the exponential histogram bucket
// Based on: https://github.com/open-telemetry/opentelemetry-go/blob/3a72c5ea94bf843beeaa044b0dda2ce4d627bb7b/sdk/metric/internal/aggregate/exponential_histogram.go#L122
// See also: https://github.com/open-telemetry/opentelemetry-go/blob/3a72c5ea94bf843beeaa044b0dda2ce4d627bb7b/sdk/metric/internal/aggregate/exponential_histogram.go#L139
func histogramLowerBound(histIndex int) float64 {
	inverseFactor := math.Ldexp(math.Ln2, -scale)
	return math.Exp(float64(histIndex) * inverseFactor)
}

// sketchLowerBoundToHistogramIndex takes the lower boundary of a sketch bucket and computes the
// closest equivalent exponential histogram index that corresponds to an exponential histogram
// bucket that has a range covering that lower bound
// See: https://opentelemetry.io/docs/specs/otel/metrics/data-model/#all-scales-use-the-logarithm-function
func sketchLowerBoundToHistogramIndex(value float64) int {
	if frac, exp := math.Frexp(value); frac == 0.5 {
		return ((exp - 1) << scale) - 1
	}
	scaleFactor := math.Ldexp(math.Log2E, scale)

	return int(math.Floor(math.Log(value) * scaleFactor))
}
