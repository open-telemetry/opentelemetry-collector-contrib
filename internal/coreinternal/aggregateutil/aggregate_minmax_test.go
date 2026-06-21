// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package aggregateutil // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/aggregateutil"

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

// When merging histogram data points, a source point that does not carry a
// min/max must not overwrite the accumulated min/max. Its Min()/Max() return 0
// when unset, which would otherwise wrongly pull the merged min down to 0 (when
// the accumulated min is positive) or the merged max up to 0 (when the
// accumulated max is negative).
func Test_mergeHistogramDataPoints_SourceWithoutMinMax(t *testing.T) {
	t.Run("min preserved", func(t *testing.T) {
		dps := pmetric.NewHistogramDataPointSlice()
		d0 := dps.AppendEmpty()
		d0.SetCount(2)
		d0.SetMin(5)
		d0.SetMax(8)
		d0.BucketCounts().Append(2)
		d1 := dps.AppendEmpty() // no Min/Max set
		d1.SetCount(3)
		d1.BucketCounts().Append(3)

		to := pmetric.NewHistogramDataPointSlice()
		mergeHistogramDataPoints(map[string]pmetric.HistogramDataPointSlice{"k": dps}, to)

		require.Equal(t, 1, to.Len())
		require.True(t, to.At(0).HasMin())
		require.Equal(t, 5.0, to.At(0).Min())
	})

	t.Run("negative max preserved", func(t *testing.T) {
		dps := pmetric.NewHistogramDataPointSlice()
		d0 := dps.AppendEmpty()
		d0.SetCount(2)
		d0.SetMin(-8)
		d0.SetMax(-3)
		d0.BucketCounts().Append(2)
		d1 := dps.AppendEmpty() // no Min/Max set
		d1.SetCount(3)
		d1.BucketCounts().Append(3)

		to := pmetric.NewHistogramDataPointSlice()
		mergeHistogramDataPoints(map[string]pmetric.HistogramDataPointSlice{"k": dps}, to)

		require.Equal(t, 1, to.Len())
		require.True(t, to.At(0).HasMax())
		require.Equal(t, -3.0, to.At(0).Max())
	})
}

func Test_mergeExponentialHistogramDataPoints_SourceWithoutMinMax(t *testing.T) {
	t.Run("min preserved", func(t *testing.T) {
		dps := pmetric.NewExponentialHistogramDataPointSlice()
		d0 := dps.AppendEmpty()
		d0.SetCount(2)
		d0.SetMin(5)
		d0.SetMax(8)
		d0.Positive().BucketCounts().Append(2)
		d1 := dps.AppendEmpty() // no Min/Max set
		d1.SetCount(3)
		d1.Positive().BucketCounts().Append(3)

		to := pmetric.NewExponentialHistogramDataPointSlice()
		mergeExponentialHistogramDataPoints(map[string]pmetric.ExponentialHistogramDataPointSlice{"k": dps}, to)

		require.Equal(t, 1, to.Len())
		require.True(t, to.At(0).HasMin())
		require.Equal(t, 5.0, to.At(0).Min())
	})

	t.Run("negative max preserved", func(t *testing.T) {
		dps := pmetric.NewExponentialHistogramDataPointSlice()
		d0 := dps.AppendEmpty()
		d0.SetCount(2)
		d0.SetMin(-8)
		d0.SetMax(-3)
		d0.Positive().BucketCounts().Append(2)
		d1 := dps.AppendEmpty() // no Min/Max set
		d1.SetCount(3)
		d1.Positive().BucketCounts().Append(3)

		to := pmetric.NewExponentialHistogramDataPointSlice()
		mergeExponentialHistogramDataPoints(map[string]pmetric.ExponentialHistogramDataPointSlice{"k": dps}, to)

		require.Equal(t, 1, to.Len())
		require.True(t, to.At(0).HasMax())
		require.Equal(t, -3.0, to.At(0).Max())
	})
}
