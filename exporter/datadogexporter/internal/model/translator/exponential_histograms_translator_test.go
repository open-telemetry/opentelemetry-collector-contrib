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

package translator // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/model/translator"

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/DataDog/datadog-agent/pkg/quantile/summary"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"
)

const (
	acceptableFloatError = 1e-12
)

func TestExponentialHistogramToDDSketch(t *testing.T) {
	ts := pdata.NewTimestampFromTime(time.Now())
	point := pdata.NewExponentialHistogramDataPoint()
	point.SetScale(6)

	point.SetCount(30)
	point.SetZeroCount(10)
	point.SetSum(math.Pi)

	point.Negative().SetOffset(2)
	point.Negative().SetBucketCounts([]uint64{3, 2, 5})

	point.Positive().SetOffset(int32(3))
	point.Positive().SetBucketCounts([]uint64{1, 1, 1, 2, 2, 3})

	point.SetTimestamp(ts)

	tr := newTranslator(t, zap.NewNop())

	sketch, err := tr.exponentialHistogramToDDSketch(point, true)
	assert.NoError(t, err)

	sketch.GetPositiveValueStore().ForEach(func(index int, count float64) bool {
		assert.Equal(t, float64(point.Positive().BucketCounts()[index-int(point.Positive().Offset())]), count)
		return false
	})

	sketch.GetNegativeValueStore().ForEach(func(index int, count float64) bool {
		assert.Equal(t, float64(point.Negative().BucketCounts()[index-int(point.Negative().Offset())]), count)
		return false
	})

	assert.Equal(t, float64(point.Count()), sketch.GetCount())

	assert.Equal(t, float64(point.ZeroCount()), sketch.GetCount()-sketch.GetPositiveValueStore().TotalCount()-sketch.GetNegativeValueStore().TotalCount())

	gamma := math.Pow(2, math.Pow(2, float64(-point.Scale())))
	accuracy := (gamma - 1) / (gamma + 1)
	assert.InDelta(t, accuracy, sketch.RelativeAccuracy(), acceptableFloatError)

}

func testMatchingSketch(t *testing.T, expected, actual sketch) {
	assert.Equal(t, expected.name, actual.name, "expected and actual sketch names do not match")
	assert.Equal(t, expected.host, actual.host, "expected and actual sketch hosts do not match")
	assert.Equal(t, expected.timestamp, actual.timestamp, "expected and actual sketch timestamps do not match")
	assert.Equal(t, expected.basic.Sum, actual.basic.Sum, "expected and actual sketch sums do not match")
	assert.Equal(t, expected.basic.Cnt, actual.basic.Cnt, "expected and actual sketch counts do not match")
	assert.Equal(t, expected.basic.Avg, actual.basic.Avg, "expected and actual sketch averages do not match")
	assert.ElementsMatch(t, expected.tags, actual.tags, "expected and actual sketch tags do not match")

	// We can't do exact comparisons for Min and Max, given that they're inferred from the sketch
	// TODO: once exact min and max are provided, use them: https://github.com/open-telemetry/opentelemetry-proto/pull/279
	// Note: 0.01 delta taken at random, may not work with any exponential histogram input
	assert.InDelta(t, actual.basic.Max, expected.basic.Max, 0.01, "expected and actual sketch maximums do not match")
	assert.InDelta(t, actual.basic.Min, expected.basic.Min, 0.01, "expected and actual sketch minimums do not match")
}

func TestMapDeltaExponentialHistogramMetrics(t *testing.T) {
	ts := pdata.NewTimestampFromTime(time.Now())
	slice := pdata.NewExponentialHistogramDataPointSlice()
	point := slice.AppendEmpty()
	point.SetScale(6)
	point.SetCount(30)
	point.Negative().SetOffset(2)
	point.Negative().SetBucketCounts([]uint64{3, 2, 5})
	point.Positive().SetOffset(3)
	point.Positive().SetBucketCounts([]uint64{7, 1, 1, 1})
	point.SetSum(math.Pi)
	point.SetZeroCount(10)
	point.SetTimestamp(ts)

	// gamma = 2^(2^-scale)
	gamma := math.Pow(2, math.Pow(2, -float64(point.Scale())))

	dims := newDims("expHist.test")
	dimsTags := dims.AddTags("attribute_tag:attribute_value")
	counts := []metric{
		newCount(dims.WithSuffix("count"), uint64(ts), 30),
		newCount(dims.WithSuffix("sum"), uint64(ts), math.Pi),
	}

	countsAttributeTags := []metric{
		newCount(dimsTags.WithSuffix("count"), uint64(ts), 30),
		newCount(dimsTags.WithSuffix("sum"), uint64(ts), math.Pi),
	}

	sketches := []sketch{
		newSketch(dims, uint64(ts), summary.Summary{
			// Expected min: lower bound of the highest negative bucket
			Min: -math.Pow(gamma, float64(int(point.Negative().Offset())+len(point.Negative().BucketCounts()))),
			// Expected max: upper bound of the highest negative bucket
			Max: math.Pow(gamma, float64(int(point.Positive().Offset())+len(point.Positive().BucketCounts()))),
			Sum: point.Sum(),
			Avg: point.Sum() / float64(point.Count()),
			Cnt: int64(point.Count()),
		}),
	}

	sketchesAttributeTags := []sketch{
		newSketch(dimsTags, uint64(ts), summary.Summary{
			// Expected min: lower bound of the highest negative bucket
			Min: -math.Pow(gamma, float64(int(point.Negative().Offset())+len(point.Negative().BucketCounts()))),
			// Expected max: upper bound of the highest negative bucket
			Max: math.Pow(gamma, float64(int(point.Positive().Offset())+len(point.Positive().BucketCounts()))),
			Sum: point.Sum(),
			Avg: point.Sum() / float64(point.Count()),
			Cnt: int64(point.Count()),
		}),
	}

	ctx := context.Background()
	delta := true

	tests := []struct {
		name             string
		sendCountSum     bool
		tags             []string
		expectedMetrics  []metric
		expectedSketches []sketch
	}{
		{
			name:             "Send count & sum metrics, no attribute tags",
			sendCountSum:     true,
			expectedMetrics:  counts,
			expectedSketches: sketches,
		},
		{
			name:             "Send count & sum metrics, attribute tags",
			sendCountSum:     true,
			tags:             []string{"attribute_tag:attribute_value"},
			expectedMetrics:  countsAttributeTags,
			expectedSketches: sketchesAttributeTags,
		},
		{
			name:             "Don't send count & sum metrics, no attribute tags",
			sendCountSum:     false,
			expectedMetrics:  []metric{},
			expectedSketches: sketches,
		},
		{
			name:             "Don't send count & sum metrics, attribute tags",
			sendCountSum:     false,
			tags:             []string{"attribute_tag:attribute_value"},
			expectedMetrics:  []metric{},
			expectedSketches: sketchesAttributeTags,
		},
	}

	for _, testInstance := range tests {
		t.Run(testInstance.name, func(t *testing.T) {
			tr := newTranslator(t, zap.NewNop())
			tr.cfg.SendCountSum = testInstance.sendCountSum
			consumer := &mockFullConsumer{}
			dims := metricsDimensions{name: "expHist.test", tags: testInstance.tags}
			tr.mapExponentialHistogramMetrics(ctx, consumer, dims, slice, delta)
			assert.ElementsMatch(t, testInstance.expectedMetrics, consumer.metrics)
			assert.Equal(t, len(testInstance.expectedSketches), len(consumer.sketches), "sketches list doesn't have the expected size")
			for i := range consumer.sketches {
				testMatchingSketch(t, testInstance.expectedSketches[i], consumer.sketches[i])
			}
		})
	}
}
