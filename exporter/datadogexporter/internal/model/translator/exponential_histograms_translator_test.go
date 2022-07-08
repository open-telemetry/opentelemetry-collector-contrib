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
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/model/attributes"
)

const (
	acceptableFloatError = 1e-12
)

func TestExponentialHistogramToDDSketch(t *testing.T) {
	ts := pcommon.NewTimestampFromTime(time.Now())
	point := pmetric.NewExponentialHistogramDataPoint()
	point.SetScale(6)

	point.SetCount(30)
	point.SetZeroCount(10)
	point.SetSum(math.Pi)

	point.Negative().SetOffset(2)
	point.Negative().SetBucketCounts(pcommon.NewImmutableUInt64Slice([]uint64{3, 2, 5}))

	point.Positive().SetOffset(3)
	point.Positive().SetBucketCounts(pcommon.NewImmutableUInt64Slice([]uint64{1, 1, 1, 2, 2, 3}))

	point.SetTimestamp(ts)

	tr := newTranslator(t, zap.NewNop())

	sketch, err := tr.exponentialHistogramToDDSketch(point, true)
	assert.NoError(t, err)

	sketch.GetPositiveValueStore().ForEach(func(index int, count float64) bool {
		expectedCount := float64(point.Positive().BucketCounts().At(index - int(point.Positive().Offset())))
		assert.Equal(t, expectedCount, count)
		return false
	})

	sketch.GetNegativeValueStore().ForEach(func(index int, count float64) bool {
		expectedCount := float64(point.Negative().BucketCounts().At(index - int(point.Negative().Offset())))
		assert.Equal(t, expectedCount, count)
		return false
	})

	assert.Equal(t, float64(point.Count()), sketch.GetCount())
	assert.Equal(t, float64(point.ZeroCount()), sketch.GetCount()-sketch.GetPositiveValueStore().TotalCount()-sketch.GetNegativeValueStore().TotalCount())

	gamma := math.Pow(2, math.Pow(2, float64(-point.Scale())))
	accuracy := (gamma - 1) / (gamma + 1)
	assert.InDelta(t, accuracy, sketch.RelativeAccuracy(), acceptableFloatError)
}

func createExponentialHistogramMetrics(additionalResourceAttributes map[string]string, additionalDatapointAttributes map[string]string) pmetric.Metrics {
	md := pmetric.NewMetrics()
	rms := md.ResourceMetrics()
	rm := rms.AppendEmpty()

	resourceAttrs := rm.Resource().Attributes()
	resourceAttrs.InsertString(attributes.AttributeDatadogHostname, testHostname)
	for attr, val := range additionalResourceAttributes {
		resourceAttrs.InsertString(attr, val)
	}

	ilms := rm.ScopeMetrics()
	ilm := ilms.AppendEmpty()
	metricsArray := ilm.Metrics()

	met := metricsArray.AppendEmpty()
	met.SetName("expHist.test")
	met.SetDataType(pmetric.MetricDataTypeExponentialHistogram)
	met.ExponentialHistogram().SetAggregationTemporality(pmetric.MetricAggregationTemporalityDelta)
	points := met.ExponentialHistogram().DataPoints()
	point := points.AppendEmpty()

	datapointAttrs := point.Attributes()
	for attr, val := range additionalDatapointAttributes {
		datapointAttrs.InsertString(attr, val)
	}

	point.SetScale(6)

	point.SetCount(30)
	point.SetZeroCount(10)
	point.SetSum(math.Pi)

	point.Negative().SetOffset(2)
	point.Negative().SetBucketCounts(pcommon.NewImmutableUInt64Slice([]uint64{3, 2, 5}))

	point.Positive().SetOffset(3)
	point.Positive().SetBucketCounts(pcommon.NewImmutableUInt64Slice([]uint64{1, 1, 1, 2, 2, 3}))

	point.SetTimestamp(seconds(0))

	return md
}

func TestMapDeltaExponentialHistogramMetrics(t *testing.T) {
	metrics := createExponentialHistogramMetrics(map[string]string{}, map[string]string{
		"attribute_tag": "attribute_value",
	})

	point := metrics.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).ExponentialHistogram().DataPoints().At(0)
	// gamma = 2^(2^-scale)
	gamma := math.Pow(2, math.Pow(2, -float64(point.Scale())))

	counts := []metric{
		newCountWithHostname("expHist.test.count", 30, uint64(seconds(0)), []string{"attribute_tag:attribute_value"}),
		newCountWithHostname("expHist.test.sum", math.Pi, uint64(seconds(0)), []string{"attribute_tag:attribute_value"}),
	}

	sketches := []sketch{
		newSketchWithHostname("expHist.test", summary.Summary{
			// Expected min: lower bound of the highest negative bucket
			Min: -math.Pow(gamma, float64(int(point.Negative().Offset())+point.Negative().BucketCounts().Len())),
			// Expected max: upper bound of the highest positive bucket
			Max: math.Pow(gamma, float64(int(point.Positive().Offset())+point.Positive().BucketCounts().Len())),
			Sum: point.Sum(),
			Avg: point.Sum() / float64(point.Count()),
			Cnt: int64(point.Count()),
		}, []string{"attribute_tag:attribute_value"}),
	}

	ctx := context.Background()

	tests := []struct {
		name             string
		sendCountSum     bool
		tags             []string
		expectedMetrics  []metric
		expectedSketches []sketch
	}{
		{
			name:             "Send count & sum metrics",
			sendCountSum:     true,
			tags:             []string{"attribute_tag:attribute_value"},
			expectedMetrics:  counts,
			expectedSketches: sketches,
		},
		{
			name:             "Don't send count & sum metrics",
			sendCountSum:     false,
			tags:             []string{"attribute_tag:attribute_value"},
			expectedMetrics:  []metric{},
			expectedSketches: sketches,
		},
	}

	for _, testInstance := range tests {
		t.Run(testInstance.name, func(t *testing.T) {
			tr := newTranslator(t, zap.NewNop())
			tr.cfg.SendCountSum = testInstance.sendCountSum
			consumer := &mockFullConsumer{}
			err := tr.MapMetrics(ctx, metrics, consumer)
			assert.NoError(t, err)
			assert.ElementsMatch(t, testInstance.expectedMetrics, consumer.metrics)
			// We don't necessarily have strict equality between expected and actual sketches
			// for ExponentialHistograms, therefore we use testMatchingSketches to compare the
			// sketches with more lenient comparisons.
			testMatchingSketches(t, testInstance.expectedSketches, consumer.sketches)
		})
	}
}
