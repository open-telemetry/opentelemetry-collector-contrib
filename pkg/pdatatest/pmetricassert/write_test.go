// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pmetricassert

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalize_CapturesValues(t *testing.T) {
	dp := normalize(buildHistogramMetrics()).Resources[0].Scopes[0].Metrics[0].Datapoints[0]
	require.Equal(t, uint64(10), *dp.Count)
	require.InDelta(t, 0.5, *dp.Sum, 0)
	require.InDelta(t, 0.001, *dp.Min, 0)
	require.InDelta(t, 0.2, *dp.Max, 0)
	require.Equal(t, []float64{0.005, 0.01, 0.025}, *dp.ExplicitBounds)
	require.Equal(t, []uint64{1, 2, 3, 4}, dp.BucketCounts)

	gauge := normalize(buildSampleMetrics()).Resources[0].Scopes[0].Metrics[0].Datapoints[0]
	require.Equal(t, int64(7), *gauge.IntValue)
}

func TestProject(t *testing.T) {
	bounds := []float64{0.005, 0.01, 0.025}
	full := normalize(buildHistogramMetrics()).Resources[0].Scopes[0].Metrics[0].Datapoints[0]

	tests := map[string]struct {
		opts writeOptions
		want datapointSnapshot
	}{
		"default keeps identity only": {
			want: datapointSnapshot{},
		},
		"explicit bounds only": {
			opts: writeOptions{includeHistogramExplicitBounds: true},
			want: datapointSnapshot{ExplicitBounds: &bounds},
		},
		"values keep every field": {
			opts: writeOptions{includeValues: true},
			want: full,
		},
		"values with explicit bounds": {
			opts: writeOptions{includeValues: true, includeHistogramExplicitBounds: true},
			want: full,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			snap := normalize(buildHistogramMetrics())
			project(snap, tt.opts)
			require.Equal(t, tt.want, snap.Resources[0].Scopes[0].Metrics[0].Datapoints[0])
		})
	}
}

func TestProject_KeepsAttributes(t *testing.T) {
	snap := normalize(buildSampleMetrics())
	project(snap, writeOptions{})

	requests := snap.Resources[0].Scopes[0].Metrics[1]
	require.Equal(t, "svc.requests", requests.Name)
	require.Equal(t, []datapointSnapshot{
		{Attributes: map[string]any{"method": "GET"}},
		{Attributes: map[string]any{"method": "POST"}},
	}, requests.Datapoints)
}
