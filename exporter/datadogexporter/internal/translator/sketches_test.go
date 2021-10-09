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
	"fmt"
	"math"
	"testing"

	"github.com/DataDog/datadog-agent/pkg/quantile"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"
)

func TestHistogramSketches(t *testing.T) {
	N := 1_000
	M := 50_000.0

	// Given a cumulative distribution function for a distribution
	// with support [0, N], generate an OTLP Histogram data point with N buckets,
	// (-inf, 0], (0, 1], ..., (N-1, N], (N, inf)
	// which contains N*M uniform samples of the distribution.
	fromCDF := func(cdf func(x float64) float64) pdata.HistogramDataPoint {
		p := pdata.NewHistogramDataPoint()
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
		p.SetExplicitBounds(bounds)
		p.SetBucketCounts(buckets)
		p.SetCount(count)
		return p
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
	tr := newTranslator(t, zap.NewNop())

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			p := fromCDF(test.cdf)
			sk := tr.getSketchBuckets("test", 0, p, true, []string{}).Points[0].Sketch

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
			for i := 0; i < len(p.BucketCounts())-3; i++ {
				{
					q := float64(cumulSum) / float64(p.Count()) * (1 - tol)
					quantileValue := sk.Quantile(cfg, q)
					// quantileValue, if computed from the explicit buckets, would have to be <= bounds[i].
					// Because of remapping, it is <= bounds[i+1].
					// Because of DDSketch accuracy guarantees, it is <= bounds[i+1] * (1 + defaultEps)
					maxExpectedQuantileValue := p.ExplicitBounds()[i+1] * (1 + defaultEps)
					assert.LessOrEqual(t, quantileValue, maxExpectedQuantileValue)
				}

				cumulSum += p.BucketCounts()[i+1]

				{
					q := float64(cumulSum) / float64(p.Count()) * (1 + tol)
					quantileValue := sk.Quantile(cfg, q)
					// quantileValue, if computed from the explicit buckets, would have to be >= bounds[i+1].
					// Because of remapping, it is >= bounds[i].
					// Because of DDSketch accuracy guarantees, it is >= bounds[i] * (1 - defaultEps)
					minExpectedQuantileValue := p.ExplicitBounds()[i] * (1 - defaultEps)
					assert.GreaterOrEqual(t, quantileValue, minExpectedQuantileValue)
				}
			}
		})
	}
}

func TestInfiniteBounds(t *testing.T) {

	tests := []struct {
		name    string
		getHist func() pdata.HistogramDataPoint
	}{
		{
			name: "(-inf, inf): 100",
			getHist: func() pdata.HistogramDataPoint {
				p := pdata.NewHistogramDataPoint()
				p.SetExplicitBounds([]float64{})
				p.SetBucketCounts([]uint64{100})
				p.SetCount(100)
				p.SetSum(0)
				return p
			},
		},
		{
			name: "(-inf, 0]: 100, (0, +inf]: 100",
			getHist: func() pdata.HistogramDataPoint {
				p := pdata.NewHistogramDataPoint()
				p.SetExplicitBounds([]float64{0})
				p.SetBucketCounts([]uint64{100, 100})
				p.SetCount(200)
				p.SetSum(0)
				return p
			},
		},
		{
			name: "(-inf, -1]: 100, (-1, 1]: 10,  (1, +inf]: 100",
			getHist: func() pdata.HistogramDataPoint {
				p := pdata.NewHistogramDataPoint()
				p.SetExplicitBounds([]float64{-1, 1})
				p.SetBucketCounts([]uint64{100, 10, 100})
				p.SetCount(210)
				p.SetSum(0)
				return p
			},
		},
	}

	tr := newTranslator(t, zap.NewNop())
	for _, testInstance := range tests {
		t.Run(testInstance.name, func(t *testing.T) {
			p := testInstance.getHist()
			sk := tr.getSketchBuckets("test", 0, p, true, []string{}).Points[0].Sketch
			assert.InDelta(t, sk.Basic.Sum, p.Sum(), 1)
			assert.Equal(t, uint64(sk.Basic.Cnt), p.Count())
		})
	}

}
