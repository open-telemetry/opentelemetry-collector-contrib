// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package expo_test

import (
	"fmt"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data/expo"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data/expo/expotest"
)

func TestAbsolute(t *testing.T) {
	is := expotest.Is(t)

	bs := expotest.Bins{ø, 1, 2, 3, 4, 5, ø, ø}.Into()
	abs := expo.Abs(bs)

	lo, up := abs.Lower(), abs.Upper()
	is.Equalf(-2, lo, "lower-bound")
	is.Equalf(3, up, "upper-bound")

	for i := lo; i < up; i++ {
		got := abs.Abs(i)
		is.Equal(bs.BucketCounts().At(i+2), got)
	}
}

func ExampleAbsolute() {
	nums := []float64{0.4, 2.3, 2.4, 4.5}

	bs := expotest.Observe0(nums...)
	abs := expo.Abs(bs)

	fmt.Printf("spans from %d:%d\n", abs.Lower(), abs.Upper())

	s := expo.Scale(0)
	for _, n := range nums {
		fmt.Printf("%.1f belongs to bucket %2d: %d\n", n, s.Idx(n), abs.Abs(s.Idx(n)))
	}

	// Output:
	// spans from -2:3
	// 0.4 belongs to bucket -2: 1
	// 2.3 belongs to bucket  1: 2
	// 2.4 belongs to bucket  1: 2
	// 4.5 belongs to bucket  2: 1
}
