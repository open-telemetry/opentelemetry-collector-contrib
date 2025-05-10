// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package expo_test

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data/datatest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data/expo"
)

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
				for _, elem := range strings.Fields(r.bkt) {
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

			is := datatest.New(t)
			for i := 0; i < len(buckets)-1; i++ {
				expo.Downscale(buckets[i].bkt, buckets[i].scale, buckets[i+1].scale)

				is.Equalf(buckets[i+1].bkt.Offset(), buckets[i].bkt.Offset(), "offset")

				want := buckets[i+1].bkt.BucketCounts().AsRaw()
				got := buckets[i].bkt.BucketCounts().AsRaw()

				is.Equalf(want, got[:len(want)], "counts")
				is.Equalf(make([]uint64, len(got)-len(want)), got[len(want):], "extra-space")
			}
		})
	}

	t.Run("panics", func(t *testing.T) {
		assert.PanicsWithValue(t, "cannot upscale without introducing error (8 -> 12)", func() {
			expo.Downscale(bins{}.Into(), 8, 12)
		})
	})
}
