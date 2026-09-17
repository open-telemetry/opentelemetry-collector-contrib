// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"testing"

	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
)

func BenchmarkConvertExponentialHistToExplicitHist(b *testing.B) {
	bounds := []float64{0, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 25, 50, 100}
	template := pmetric.NewMetric()
	dp := template.SetEmptyExponentialHistogram().DataPoints().AppendEmpty()
	dp.SetScale(4)
	dp.SetZeroCount(10)
	dp.Positive().SetOffset(-80)
	var count uint64 = 10
	for i := range 160 {
		bucketCount := uint64(i%7 + 1)
		dp.Positive().BucketCounts().Append(bucketCount)
		count += bucketCount
	}
	dp.SetCount(count)
	dp.Attributes().PutStr("service.name", "benchmark")

	for _, distribution := range []string{"upper", "midpoint", "uniform", "random"} {
		b.Run(distribution, func(b *testing.B) {
			expr, err := convertExponentialHistToExplicitHist(distribution, bounds)
			if err != nil {
				b.Fatal(err)
			}
			metric := pmetric.NewMetric()
			resourceMetrics := pmetric.NewResourceMetrics()
			scopeMetrics := pmetric.NewScopeMetrics()
			transformContext := ottlmetric.NewTransformContext(resourceMetrics, scopeMetrics, metric)
			b.Cleanup(transformContext.Close)
			b.ReportAllocs()
			for b.Loop() {
				template.CopyTo(metric)
				_, err = expr(b.Context(), transformContext)
				if err != nil {
					b.Fatal(err)
				}
			}
			if got := metric.Histogram().DataPoints().At(0).Count(); got != count {
				b.Fatalf("converted count = %d, want %d", got, count)
			}
			b.ReportMetric(float64(dp.Positive().BucketCounts().Len()), "source-buckets/op")
		})
	}
}
