// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics_test

import (
	"math/rand/v2"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	conventions "go.opentelemetry.io/otel/semconv/v1.27.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

func TestMergeMetrics(t *testing.T) {
	t.Parallel()

	testCases := []string{
		"basic_merge",
		"a_duplicate_data",
	}

	for _, tc := range testCases {
		testName := tc

		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			dir := filepath.Join("testdata", testName)

			mdA, err := golden.ReadMetrics(filepath.Join(dir, "a.yaml"))
			require.NoError(t, err)

			mdB, err := golden.ReadMetrics(filepath.Join(dir, "b.yaml"))
			require.NoError(t, err)

			expectedOutput, err := golden.ReadMetrics(filepath.Join(dir, "output.yaml"))
			require.NoError(t, err)

			output := metrics.Merge(mdA, mdB)
			require.NoError(t, pmetrictest.CompareMetrics(expectedOutput, output))
		})
	}
}

func naiveMerge(mdA pmetric.Metrics, mdB pmetric.Metrics) pmetric.Metrics {
	for i := 0; i < mdB.ResourceMetrics().Len(); i++ {
		rm := mdB.ResourceMetrics().At(i)

		rmCopy := mdA.ResourceMetrics().AppendEmpty()
		rm.CopyTo(rmCopy)
	}

	return mdA
}

func BenchmarkMergeManyIntoSingle(b *testing.B) {
	benchmarks := []struct {
		name      string
		mergeFunc func(mdA pmetric.Metrics, mdB pmetric.Metrics) pmetric.Metrics
	}{
		{
			name:      "Naive",
			mergeFunc: naiveMerge,
		},
		{
			name:      "Deduplicating",
			mergeFunc: metrics.Merge,
		},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			// Make mdA just be a single resource metric with a single scope metric and a single metric
			mdAClean := generateMetrics(b, 1)
			mdB := generateMetrics(b, 10000)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				mdA := pmetric.NewMetrics()
				mdAClean.CopyTo(mdA)
				b.StartTimer()

				bm.mergeFunc(mdA, mdB)
			}
		})
	}
}

func BenchmarkMergeManyIntoMany(b *testing.B) {
	benchmarks := []struct {
		name      string
		mergeFunc func(mdA pmetric.Metrics, mdB pmetric.Metrics) pmetric.Metrics
	}{
		{
			name:      "Naive",
			mergeFunc: naiveMerge,
		},
		{
			name:      "Deduplicating",
			mergeFunc: metrics.Merge,
		},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			mdAClean := generateMetrics(b, 10000)
			mdB := generateMetrics(b, 10000)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				mdA := pmetric.NewMetrics()
				mdAClean.CopyTo(mdA)
				b.StartTimer()

				bm.mergeFunc(mdA, mdB)
			}
		})
	}
}

// generateMetrics creates a pmetric.Metrics instance with `rmCount` resourceMetrics.
// Each resource metric is the same as the others, each scope metric is the same
// as the others, each metric is the same as the others. But the datapoints are different
func generateMetrics(t require.TestingT, rmCount int) pmetric.Metrics {
	md := pmetric.NewMetrics()

	timeStamp := pcommon.Timestamp(rand.IntN(256))
	value := rand.Int64N(256)

	for i := 0; i < rmCount; i++ {
		rm := md.ResourceMetrics().AppendEmpty()
		err := rm.Resource().Attributes().FromRaw(map[string]any{
			string(conventions.ServiceNameKey): "service-test",
		})
		require.NoError(t, err)

		sm := rm.ScopeMetrics().AppendEmpty()
		scope := sm.Scope()
		scope.SetName("MyTestInstrument")
		scope.SetVersion("1.2.3")
		err = scope.Attributes().FromRaw(map[string]any{
			"scope.key": "scope-test",
		})
		require.NoError(t, err)

		m := sm.Metrics().AppendEmpty()
		m.SetName("metric.test")

		sum := m.SetEmptySum()
		sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
		sum.SetIsMonotonic(true)

		dp := sum.DataPoints().AppendEmpty()

		dp.SetTimestamp(timeStamp)
		timeStamp += 10

		dp.SetIntValue(value)
		value += 15

		err = dp.Attributes().FromRaw(map[string]any{
			"datapoint.key": "dp-test",
		})
		require.NoError(t, err)
	}

	return md
}
