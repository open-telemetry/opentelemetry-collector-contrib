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

package translator

import (
	"context"
	"fmt"
	"math"
	"testing"

	"github.com/DataDog/datadog-agent/pkg/quantile"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

var _ SketchConsumer = (*sketchConsumer)(nil)

type sketchConsumer struct {
	mockTimeSeriesConsumer
	sk *quantile.Sketch
}

// ConsumeSketch implements the translator.Consumer interface.
func (c *sketchConsumer) ConsumeSketch(
	_ context.Context,
	_ *Dimensions,
	_ uint64,
	sketch *quantile.Sketch,
) {
	c.sk = sketch
}

func newHistogramMetric(p pmetric.HistogramDataPoint) pmetric.Metrics {
	md := pmetric.NewMetrics()
	rms := md.ResourceMetrics()
	rm := rms.AppendEmpty()
	ilms := rm.ScopeMetrics()
	ilm := ilms.AppendEmpty()
	metricsArray := ilm.Metrics()
	m := metricsArray.AppendEmpty()
	m.SetDataType(pmetric.MetricDataTypeHistogram)
	m.SetName("test")

	// Copy Histogram point
	m.Histogram().SetAggregationTemporality(pmetric.MetricAggregationTemporalityDelta)
	dps := m.Histogram().DataPoints()
	np := dps.AppendEmpty()
	np.SetCount(p.Count())
	np.SetSum(p.Sum())
	np.SetMBucketCounts(p.MBucketCounts())
	np.SetMExplicitBounds(p.MExplicitBounds())
	np.SetTimestamp(p.Timestamp())

	return md
}

func TestHistogramSketches(t *testing.T) {
	N := 1_000
	M := 50_000.0

	// Given a cumulative distribution function for a distribution
	// with support [0, N], generate an OTLP Histogram data point with N buckets,
	// (-inf, 0], (0, 1], ..., (N-1, N], (N, inf)
	// which contains N*M uniform samples of the distribution.
	fromCDF := func(cdf func(x float64) float64) pmetric.Metrics {
		p := pmetric.NewHistogramDataPoint()
		bounds := make([]float64, N+1)
		buckets := make([]uint64, N+2)
		buckets[0] = 0
		count := uint64(0)
		for i := 0; i < N; i++ {
			bounds[i] = float64(i)
			// the bucket with bounds (i, i+1) has the
			// cdf delta between the bounds as a value.
			buckets[i+1] = uint64((cdf(float64(i+1)) - cdf(float64(i))) * M)
			count += buckets[i+1]
		}
		bounds[N] = float64(N)
		buckets[N+1] = 0
		p.SetExplicitBounds(pcommon.NewImmutableFloat64Slice(bounds))
		p.SetBucketCounts(pcommon.NewImmutableUInt64Slice(buckets))
		p.SetCount(count)
		return newHistogramMetric(p)
	}

	tests := []struct {
		// distribution name
		name string
		// the cumulative distribution function (within [0,N])
		cdf func(x float64) float64
		// error tolerance for testing cdf(quantile(q)) ≈ q
		epsilon float64
	}{
		{
			// https://en.wikipedia.org/wiki/Continuous_uniform_distribution
			name:    "Uniform distribution (a=0,b=N)",
			cdf:     func(x float64) float64 { return x / float64(N) },
			epsilon: 0.01,
		},
		{
			// https://en.wikipedia.org/wiki/U-quadratic_distribution
			name: "U-quadratic distribution (a=0,b=N)",
			cdf: func(x float64) float64 {
				a := 0.0
				b := float64(N)
				alpha := 12.0 / math.Pow(b-a, 3)
				beta := (b + a) / 2.0
				return alpha / 3 * (math.Pow(x-beta, 3) + math.Pow(beta-alpha, 3))
			},
			epsilon: 0.025,
		},
	}

	defaultEps := 1.0 / 128.0
	tol := 1e-8
	cfg := quantile.Default()
	ctx := context.Background()
	tr := newTranslator(t, zap.NewNop())
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			md := fromCDF(test.cdf)
			consumer := &sketchConsumer{}
			assert.NoError(t, tr.MapMetrics(ctx, md, consumer))
			sk := consumer.sk

			// Check the minimum is 0.0
			assert.Equal(t, 0.0, sk.Quantile(cfg, 0))
			// Check the quantiles are approximately correct
			for i := 1; i <= 99; i++ {
				q := (float64(i)) / 100.0
				assert.InEpsilon(t,
					// test that the CDF is the (approximate) inverse of the quantile function
					test.cdf(sk.Quantile(cfg, q)),
					q,
					test.epsilon,
					fmt.Sprintf("error too high for p%d", i),
				)
			}

			cumulSum := uint64(0)
			p := md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Histogram().DataPoints().At(0)
			for i := 0; i < p.BucketCounts().Len()-3; i++ {
				{
					q := float64(cumulSum) / float64(p.Count()) * (1 - tol)
					quantileValue := sk.Quantile(cfg, q)
					// quantileValue, if computed from the explicit buckets, would have to be <= bounds[i].
					// Because of remapping, it is <= bounds[i+1].
					// Because of DDSketch accuracy guarantees, it is <= bounds[i+1] * (1 + defaultEps)
					maxExpectedQuantileValue := p.ExplicitBounds().At(i+1) * (1 + defaultEps)
					assert.LessOrEqual(t, quantileValue, maxExpectedQuantileValue)
				}

				cumulSum += p.BucketCounts().At(i + 1)

				{
					q := float64(cumulSum) / float64(p.Count()) * (1 + tol)
					quantileValue := sk.Quantile(cfg, q)
					// quantileValue, if computed from the explicit buckets, would have to be >= bounds[i+1].
					// Because of remapping, it is >= bounds[i].
					// Because of DDSketch accuracy guarantees, it is >= bounds[i] * (1 - defaultEps)
					minExpectedQuantileValue := p.ExplicitBounds().At(i) * (1 - defaultEps)
					assert.GreaterOrEqual(t, quantileValue, minExpectedQuantileValue)
				}
			}
		})
	}
}

func TestExactSumCount(t *testing.T) {
	tests := []struct {
		name    string
		getHist func() pmetric.Metrics
		sum     float64
		count   uint64
	}{}

	// Add tests for issue 6129: https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/6129
	tests = append(tests,
		struct {
			name    string
			getHist func() pmetric.Metrics
			sum     float64
			count   uint64
		}{
			name: "Uniform distribution (delta)",
			getHist: func() pmetric.Metrics {
				md := pmetric.NewMetrics()
				rms := md.ResourceMetrics()
				rm := rms.AppendEmpty()
				ilms := rm.ScopeMetrics()
				ilm := ilms.AppendEmpty()
				metricsArray := ilm.Metrics()
				m := metricsArray.AppendEmpty()
				m.SetDataType(pmetric.MetricDataTypeHistogram)
				m.SetName("test")
				m.Histogram().SetAggregationTemporality(pmetric.MetricAggregationTemporalityDelta)
				dp := m.Histogram().DataPoints()
				p := dp.AppendEmpty()
				p.SetExplicitBounds(pcommon.NewImmutableFloat64Slice([]float64{0, 5_000, 10_000, 15_000,
					20_000}))
				// Points from contrib issue 6129: 0, 5_000, 10_000, 15_000, 20_000
				p.SetBucketCounts(pcommon.NewImmutableUInt64Slice([]uint64{0, 1, 1, 1, 1, 1}))
				p.SetCount(5)
				p.SetSum(50_000)
				return md
			},
			sum:   50_000,
			count: 5,
		},

		struct {
			name    string
			getHist func() pmetric.Metrics
			sum     float64
			count   uint64
		}{
			name: "Uniform distribution (cumulative)",
			getHist: func() pmetric.Metrics {
				md := pmetric.NewMetrics()
				rms := md.ResourceMetrics()
				rm := rms.AppendEmpty()
				ilms := rm.ScopeMetrics()
				ilm := ilms.AppendEmpty()
				metricsArray := ilm.Metrics()
				m := metricsArray.AppendEmpty()
				m.SetDataType(pmetric.MetricDataTypeHistogram)
				m.SetName("test")
				m.Histogram().SetAggregationTemporality(pmetric.MetricAggregationTemporalityCumulative)
				dp := m.Histogram().DataPoints()
				// Points from contrib issue 6129: 0, 5_000, 10_000, 15_000, 20_000 repeated.
				bounds := pcommon.NewImmutableFloat64Slice([]float64{0, 5_000, 10_000, 15_000, 20_000})
				for i := 1; i <= 2; i++ {
					p := dp.AppendEmpty()
					p.SetExplicitBounds(bounds)
					cnt := uint64(i)
					p.SetBucketCounts(pcommon.NewImmutableUInt64Slice([]uint64{0, cnt, cnt, cnt, cnt, cnt}))
					p.SetCount(uint64(5 * i))
					p.SetSum(float64(50_000 * i))
				}
				return md
			},
			sum:   50_000,
			count: 5,
		})

	// Add tests for issue 7065: https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/7065
	for pos, val := range []float64{500, 5_000, 50_000} {
		pos := pos
		val := val
		tests = append(tests, struct {
			name    string
			getHist func() pmetric.Metrics
			sum     float64
			count   uint64
		}{
			name: fmt.Sprintf("Issue 7065 (%d, %f)", pos, val),
			getHist: func() pmetric.Metrics {
				md := pmetric.NewMetrics()
				rms := md.ResourceMetrics()
				rm := rms.AppendEmpty()
				ilms := rm.ScopeMetrics()
				ilm := ilms.AppendEmpty()
				metricsArray := ilm.Metrics()
				m := metricsArray.AppendEmpty()
				m.SetDataType(pmetric.MetricDataTypeHistogram)
				m.SetName("test")

				m.Histogram().SetAggregationTemporality(pmetric.MetricAggregationTemporalityCumulative)
				bounds := pcommon.NewImmutableFloat64Slice([]float64{1_000, 10_000, 100_000})

				dp := m.Histogram().DataPoints()
				for i := 0; i < 2; i++ {
					p := dp.AppendEmpty()
					p.SetExplicitBounds(bounds)
					counts := []uint64{0, 0, 0, 0}
					counts[pos] = uint64(i)
					t.Logf("pos: %d, val: %f, counts: %v", pos, val, counts)
					p.SetBucketCounts(pcommon.NewImmutableUInt64Slice(counts))
					p.SetCount(uint64(i))
					p.SetSum(val * float64(i))
				}
				return md
			},
			sum:   val,
			count: 1,
		})
	}

	ctx := context.Background()
	tr := newTranslator(t, zap.NewNop())
	for _, testInstance := range tests {
		t.Run(testInstance.name, func(t *testing.T) {
			md := testInstance.getHist()
			consumer := &sketchConsumer{}
			assert.NoError(t, tr.MapMetrics(ctx, md, consumer))
			sk := consumer.sk

			assert.Equal(t, testInstance.count, uint64(sk.Basic.Cnt), "counts differ")
			assert.Equal(t, testInstance.sum, sk.Basic.Sum, "sums differ")
			avg := testInstance.sum / float64(testInstance.count)
			assert.Equal(t, avg, sk.Basic.Avg, "averages differ")
		})
	}
}

func TestInfiniteBounds(t *testing.T) {

	tests := []struct {
		name    string
		getHist func() pmetric.Metrics
	}{
		{
			name: "(-inf, inf): 100",
			getHist: func() pmetric.Metrics {
				p := pmetric.NewHistogramDataPoint()
				p.SetExplicitBounds(pcommon.NewImmutableFloat64Slice([]float64{}))
				p.SetBucketCounts(pcommon.NewImmutableUInt64Slice([]uint64{100}))
				p.SetCount(100)
				p.SetSum(0)
				return newHistogramMetric(p)
			},
		},
		{
			name: "(-inf, 0]: 100, (0, +inf]: 100",
			getHist: func() pmetric.Metrics {
				p := pmetric.NewHistogramDataPoint()
				p.SetExplicitBounds(pcommon.NewImmutableFloat64Slice([]float64{0}))
				p.SetBucketCounts(pcommon.NewImmutableUInt64Slice([]uint64{100, 100}))
				p.SetCount(200)
				p.SetSum(0)
				return newHistogramMetric(p)
			},
		},
		{
			name: "(-inf, -1]: 100, (-1, 1]: 10,  (1, +inf]: 100",
			getHist: func() pmetric.Metrics {
				p := pmetric.NewHistogramDataPoint()
				p.SetExplicitBounds(pcommon.NewImmutableFloat64Slice([]float64{-1, 1}))
				p.SetBucketCounts(pcommon.NewImmutableUInt64Slice([]uint64{100, 10, 100}))
				p.SetCount(210)
				p.SetSum(0)
				return newHistogramMetric(p)
			},
		},
	}

	ctx := context.Background()
	tr := newTranslator(t, zap.NewNop())
	for _, testInstance := range tests {
		t.Run(testInstance.name, func(t *testing.T) {
			md := testInstance.getHist()
			consumer := &sketchConsumer{}
			assert.NoError(t, tr.MapMetrics(ctx, md, consumer))
			sk := consumer.sk

			p := md.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Histogram().DataPoints().At(0)
			assert.InDelta(t, sk.Basic.Sum, p.Sum(), 1)
			assert.Equal(t, uint64(sk.Basic.Cnt), p.Count())
		})
	}

}
