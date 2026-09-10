// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

func TestMergerMetrics(t *testing.T) {
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

			merger := metrics.NewMerger(mdA)
			merger.Merge(mdB)
			require.NoError(t, pmetrictest.CompareMetrics(expectedOutput, merger.Metrics()))
		})
	}
}

// TestMergerMatchesMerge verifies that N incremental Merger.Merge calls produce
// the same result as N sequential Merge calls, across empty and pre-populated
// destinations.
func TestMergerMatchesMerge(t *testing.T) {
	t.Parallel()

	for _, destCount := range []int{0, 1, 100} {
		t.Run(fmt.Sprintf("dest_%d", destCount), func(t *testing.T) {
			t.Parallel()

			sources := make([]pmetric.Metrics, 50)
			for i := range sources {
				sources[i] = generateStreamMetrics(t, i%7, i)
			}

			destClean := generateMetrics(t, destCount)

			viaMerge := pmetric.NewMetrics()
			destClean.CopyTo(viaMerge)
			for _, src := range sources {
				metrics.Merge(viaMerge, src)
			}

			destForMerger := pmetric.NewMetrics()
			destClean.CopyTo(destForMerger)
			merger := metrics.NewMerger(destForMerger)
			for _, src := range sources {
				merger.Merge(src)
			}

			require.NoError(t, pmetrictest.CompareMetrics(viaMerge, merger.Metrics()))
		})
	}
}

// generateStreamMetrics builds a single-datapoint pmetric.Metrics whose
// resource/scope/metric identities are drawn from a small set, so merging many
// of them exercises both the "append new" and "merge into existing" paths at
// every level.
func generateStreamMetrics(t require.TestingT, resourceKey, seq int) pmetric.Metrics {
	md := pmetric.NewMetrics()

	rm := md.ResourceMetrics().AppendEmpty()
	err := rm.Resource().Attributes().FromRaw(map[string]any{
		"service.name": fmt.Sprintf("service-%d", resourceKey),
	})
	require.NoError(t, err)

	sm := rm.ScopeMetrics().AppendEmpty()
	sm.Scope().SetName(fmt.Sprintf("scope-%d", seq%3))

	m := sm.Metrics().AppendEmpty()
	m.SetName(fmt.Sprintf("metric-%d", seq%5))

	sum := m.SetEmptySum()
	sum.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
	sum.SetIsMonotonic(true)

	dp := sum.DataPoints().AppendEmpty()
	dp.SetTimestamp(pcommon.Timestamp(seq))
	dp.SetIntValue(int64(seq))
	err = dp.Attributes().FromRaw(map[string]any{
		"stream": fmt.Sprintf("stream-%d", seq),
	})
	require.NoError(t, err)

	return md
}

// BenchmarkMergeManySmallSources reproduces the loadbalancing exporter's
// metrics hot path: a large number of small (single-stream) Metrics merged one
// by one into a destination that keeps growing. Merge re-hashes the whole
// destination on every call (quadratic overall); Merger caches identities.
func BenchmarkMergeManySmallSources(b *testing.B) {
	const sourceCount = 2000

	sources := make([]pmetric.Metrics, sourceCount)
	for i := range sources {
		// ~sourceCount/4 distinct resources so the destination grows large.
		sources[i] = generateStreamMetrics(b, i%(sourceCount/4), i)
	}

	b.Run("Merge", func(b *testing.B) {
		for b.Loop() {
			dest := pmetric.NewMetrics()
			for _, src := range sources {
				metrics.Merge(dest, src)
			}
		}
	})

	b.Run("Merger", func(b *testing.B) {
		for b.Loop() {
			merger := metrics.NewMerger(pmetric.NewMetrics())
			for _, src := range sources {
				merger.Merge(src)
			}
		}
	})
}
