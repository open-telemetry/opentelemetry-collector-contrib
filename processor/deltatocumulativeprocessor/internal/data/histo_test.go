// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package data

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data/datatest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data/histo"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data/histo/histotest"
)

func TestHistoAdd(t *testing.T) {
	type histdp = histotest.Histogram
	obs := histotest.Bounds(histo.DefaultBounds).Observe

	cases := []struct {
		name   string
		dp, in histdp
		want   histdp
		flip   bool
	}{{
		name: "noop",
	}, {
		name: "simple",
		dp:   obs(-12, 5.5, 7.3, 43.3, 412.4 /*              */),
		in:   obs( /*                      */ 4.3, 14.5, 2677.4),
		want: obs(-12, 5.5, 7.3, 43.3, 412.4, 4.3, 14.5, 2677.4),
	}, {
		name: "diff-len",
		dp:   histdp{Buckets: []uint64{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}, Count: 11},
		in:   histdp{Buckets: []uint64{1, 1, 1, 1, 1 /*             */}, Count: 5},
		want: histdp{Buckets: []uint64{2, 2, 2, 2, 2, 1, 1, 1, 1, 1, 1}, Count: 11 + 5},
	}, {
		name: "diff-bounds",
		dp:   histotest.Bounds{12, 17}.Observe(3, 14, 187),
		in:   histotest.Bounds{34, 55}.Observe(8, 77, 142),
		want: histotest.Bounds{34, 55}.Observe(8, 77, 142),
	}, {
		name: "no-counts",
		dp:   histdp{Count: 42 /**/, Sum: ptr(777.12 /*   */), Min: ptr(12.3), Max: ptr(66.8)},
		in:   histdp{Count: /**/ 33, Sum: ptr( /*   */ 568.2), Min: ptr(8.21), Max: ptr(23.6)},
		want: histdp{Count: 42 + 33, Sum: ptr(777.12 + 568.2), Min: ptr(8.21), Max: ptr(66.8)},
	}, {
		name: "optional-missing",
		dp:   histdp{Count: 42 /**/, Sum: ptr(777.0) /*   */, Min: ptr(12.3), Max: ptr(66.8)},
		in:   histdp{Count: /**/ 33},
		want: histdp{Count: 42 + 33},
	}}

	for _, cs := range cases {
		t.Run(cs.name, func(t *testing.T) {
			var add Adder
			is := datatest.New(t)

			var (
				dp   = cs.dp.Into()
				in   = cs.in.Into()
				want = cs.want.Into()
			)

			err := add.Histograms(dp, in)
			is.Equal(nil, err)
			is.Equal(want, dp)
		})
	}
}

func ptr[T any](v T) *T {
	return &v
}
