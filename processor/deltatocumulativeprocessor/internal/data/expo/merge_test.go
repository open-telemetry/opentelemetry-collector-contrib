// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package expo_test

import (
	"fmt"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data/datatest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data/expo"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data/expo/expotest"
)

const ø = expotest.Empty

type bins = expotest.Bins

func TestMerge(t *testing.T) {
	cases := []struct {
		a, b bins
		want bins
	}{{
		//         -3 -2 -1 0  1  2  3  4
		a:    bins{ø, ø, ø, ø, ø, ø, ø, ø},
		b:    bins{ø, ø, ø, ø, ø, ø, ø, ø},
		want: bins{ø, ø, ø, ø, ø, ø, ø, ø},
	}, {
		a:    bins{ø, ø, 1, 1, 1, ø, ø, ø},
		b:    bins{ø, 1, 1, ø, ø, ø, ø, ø},
		want: bins{ø, 1, 2, 1, 1, ø, ø, ø},
	}, {
		a:    bins{ø, ø, ø, ø, 1, 1, 1, ø},
		b:    bins{ø, ø, ø, ø, 1, 1, 1, ø},
		want: bins{ø, ø, ø, ø, 2, 2, 2, ø},
	}, {
		a:    bins{ø, 1, 1, ø, ø, ø, ø, ø},
		b:    bins{ø, ø, ø, ø, 1, 1, ø, ø},
		want: bins{ø, 1, 1, 0, 1, 1, ø, ø},
	}}

	for _, cs := range cases {
		a := cs.a.Into()
		b := cs.b.Into()
		want := cs.want.Into()

		name := fmt.Sprintf("(%+d,%d)+(%+d,%d)=(%+d,%d)", a.Offset(), a.BucketCounts().Len(), b.Offset(), b.BucketCounts().Len(), want.Offset(), want.BucketCounts().Len())
		t.Run(name, func(t *testing.T) {
			expo.Merge(a, b)
			is := datatest.New(t)
			is.Equal(want, a)
		})
	}
}
