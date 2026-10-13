// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package expo_test

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data/expo"
)

func TestLimit(t *testing.T) {
	t.Parallel()

	const maxBuckets = 160
	cases := []struct {
		name        string
		lo, hi      int32
		wantScale   expo.Scale
		wantBuckets int
	}{
		{name: "within_limit", lo: 1, hi: 160, wantScale: 0, wantBuckets: 160},
		{name: "aligned", lo: 0, hi: 319, wantScale: -1, wantBuckets: 160},
		{name: "positive_odd_upper", lo: 0, hi: 320, wantScale: -2, wantBuckets: 81},
		{name: "positive_odd_bounds", lo: 1, hi: 320, wantScale: -2, wantBuckets: 81},
		{name: "negative_odd_lower", lo: -321, hi: -2, wantScale: -2, wantBuckets: 81},
		{name: "negative_odd_upper", lo: -320, hi: -2, wantScale: -1, wantBuckets: 160},
		{name: "cross_zero", lo: -161, hi: 160, wantScale: -2, wantBuckets: 82},
		{name: "multiple_scales", lo: 1, hi: 1280, wantScale: -4, wantBuckets: 81},
	}
	for _, cs := range cases {
		t.Run(cs.name, func(t *testing.T) {
			t.Parallel()

			// Two observations at the endpoints require all intervening buckets.
			a := pmetric.NewExponentialHistogramDataPointBuckets()
			a.SetOffset(cs.lo)
			a.BucketCounts().Append(1)
			b := pmetric.NewExponentialHistogramDataPointBuckets()
			b.SetOffset(cs.hi)
			b.BucketCounts().Append(1)

			to := expo.Limit(maxBuckets, 0, a, b)
			assert.Equal(t, cs.wantScale, to)
			assert.Equal(t, to, expo.Limit(maxBuckets, 0, b, a))

			expo.Downscale(a, 0, to)
			expo.Downscale(b, 0, to)
			expo.Merge(a, b)

			counts := a.BucketCounts()
			assert.LessOrEqual(t, counts.Len(), maxBuckets)
			require.Equal(t, cs.wantBuckets, counts.Len())
			assert.Equal(t, uint64(1), counts.At(0))
			assert.Equal(t, uint64(1), counts.At(counts.Len()-1))
			var total uint64
			for _, count := range counts.All() {
				total += count
			}
			assert.Equal(t, uint64(2), total)
		})
	}
}

func TestDownscale(t *testing.T) {
	type Repr[T any] struct {
		scale expo.Scale
		bkt   T
	}

	cases := [][]Repr[string]{{
		{scale: 2, bkt: "1 1 1 1 1 1 1 1 1 1 1 1"},
		{scale: 1, bkt: " 2   2   2   2   2   2 "},
		{scale: 0, bkt: "   4       4       4   "},
	}, {
		{scale: 2, bkt: "ø 1 1 1 1 1 1 1 1 1 1 1"},
		{scale: 1, bkt: " 1   2   2   2   2   2 "},
		{scale: 0, bkt: "   3       4       4   "},
	}, {
		{scale: 2, bkt: "ø ø 1 1 1 1 1 1 1 1 1 1"},
		{scale: 1, bkt: " ø   2   2   2   2   2 "},
		{scale: 0, bkt: "   2       4       4   "},
	}, {
		{scale: 2, bkt: "ø ø ø ø 1 1 1 1 1 1 1 1"},
		{scale: 1, bkt: " ø   ø   2   2   2   2 "},
		{scale: 0, bkt: "   ø       4       4   "},
	}, {
		{scale: 2, bkt: "1 1 1 1 1 1 1 1 1      "},
		{scale: 1, bkt: " 2   2   2   2   1     "},
		{scale: 0, bkt: "   4       4       1   "},
	}, {
		{scale: 2, bkt: "1 1 1 1 1 1 1 1 1 1 1 1"},
		{scale: 0, bkt: "   4       4       4   "},
	}, {
		{scale: 1, bkt: "ø 1 1 0"},
		{scale: 0, bkt: " 1   1 "},
	}, {
		{scale: 1, bkt: "ø 1 1 "},
		{scale: 0, bkt: " 1   1"},
	}, {
		{scale: 1, bkt: " - 1 1 "},
		{scale: 0, bkt: "- 1   1"},
	}, {
		{scale: 5, bkt: "-  4 0 3 0 3 0 0 8   "},
		{scale: 4, bkt: "- 4   3   3   0   8  "},
	}}

	type B = expo.Buckets
	for i, reprs := range cases {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			buckets := make([]Repr[B], len(reprs))
			for i, r := range reprs {
				bkt := pmetric.NewExponentialHistogramDataPointBuckets()
				for elem := range strings.FieldsSeq(r.bkt) {
					if elem == "ø" {
						bkt.SetOffset(bkt.Offset() + 1)
						continue
					}
					if elem == "-" {
						bkt.SetOffset(bkt.Offset() - 1)
						continue
					}
					n, err := strconv.Atoi(elem)
					if err != nil {
						panic(err)
					}
					bkt.BucketCounts().Append(uint64(n))
				}
				buckets[i] = Repr[B]{scale: r.scale, bkt: bkt}
			}

			for i := 0; i < len(buckets)-1; i++ {
				expo.Downscale(buckets[i].bkt, buckets[i].scale, buckets[i+1].scale)

				assert.Equal(t, buckets[i+1].bkt.Offset(), buckets[i].bkt.Offset(), "offset")

				want := buckets[i+1].bkt.BucketCounts().AsRaw()
				got := buckets[i].bkt.BucketCounts().AsRaw()

				assert.Equal(t, want, got[:len(want)], "counts")
				assert.Equal(t, make([]uint64, len(got)-len(want)), got[len(want):], "extra-space")
			}
		})
	}

	t.Run("panics", func(t *testing.T) {
		assert.PanicsWithValue(t, "cannot upscale without introducing error (8 -> 12)", func() {
			expo.Downscale(bins{}.Into(), 8, 12)
		})
	})

	t.Run("empty-buckets", func(t *testing.T) {
		t.Run("odd-offset", func(t *testing.T) {
			buckets := pmetric.NewExponentialHistogramDataPointBuckets()
			buckets.SetOffset(1)

			assert.NotPanics(t, func() {
				expo.Downscale(buckets, 2, 1)
			})

			assert.Equal(t, int32(0), buckets.Offset())
			assert.Equal(t, 0, buckets.BucketCounts().Len())
		})

		t.Run("even-offset", func(t *testing.T) {
			buckets := pmetric.NewExponentialHistogramDataPointBuckets()
			buckets.SetOffset(2)

			assert.NotPanics(t, func() {
				expo.Downscale(buckets, 2, 1)
			})

			assert.Equal(t, int32(1), buckets.Offset())
			assert.Equal(t, 0, buckets.BucketCounts().Len())
		})

		t.Run("zero-offset", func(t *testing.T) {
			buckets := pmetric.NewExponentialHistogramDataPointBuckets()
			buckets.SetOffset(0)

			assert.NotPanics(t, func() {
				expo.Downscale(buckets, 2, 1)
			})

			assert.Equal(t, int32(0), buckets.Offset())
			assert.Equal(t, 0, buckets.BucketCounts().Len())
		})

		t.Run("negative-offset-odd", func(t *testing.T) {
			buckets := pmetric.NewExponentialHistogramDataPointBuckets()
			buckets.SetOffset(-3)

			assert.NotPanics(t, func() {
				expo.Downscale(buckets, 2, 1)
			})

			assert.Equal(t, int32(-2), buckets.Offset())
			assert.Equal(t, 0, buckets.BucketCounts().Len())
		})

		t.Run("negative-offset-even", func(t *testing.T) {
			buckets := pmetric.NewExponentialHistogramDataPointBuckets()
			buckets.SetOffset(-4)

			assert.NotPanics(t, func() {
				expo.Downscale(buckets, 2, 1)
			})

			assert.Equal(t, int32(-2), buckets.Offset())
			assert.Equal(t, 0, buckets.BucketCounts().Len())
		})

		t.Run("multiple-scales", func(t *testing.T) {
			buckets := pmetric.NewExponentialHistogramDataPointBuckets()
			buckets.SetOffset(7)

			assert.NotPanics(t, func() {
				expo.Downscale(buckets, 5, 0)
			})

			assert.Equal(t, int32(0), buckets.Offset())
			assert.Equal(t, 0, buckets.BucketCounts().Len())
		})
	})
}
