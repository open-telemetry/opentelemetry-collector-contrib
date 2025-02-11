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

func TestAbsolute(t *testing.T) {
	is := datatest.New(t)

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

	s := expo.Scale(0)
	for _, n := range nums {
		fmt.Printf("%.1f belongs to bucket %+d\n", n, s.Idx(n))
	}

	fmt.Printf("\n index:")
	for i := 0; i < bs.BucketCounts().Len(); i++ {
		fmt.Printf("  %d", i)
	}
	fmt.Printf("\n   abs:")
	for i := abs.Lower(); i < abs.Upper(); i++ {
		fmt.Printf(" %+d", i)
	}
	fmt.Printf("\ncounts:")
	for i := abs.Lower(); i < abs.Upper(); i++ {
		fmt.Printf("  %d", abs.Abs(i))
	}

	// Output:
	// 0.4 belongs to bucket -2
	// 2.3 belongs to bucket +1
	// 2.4 belongs to bucket +1
	// 4.5 belongs to bucket +2
	//
	//  index:  0  1  2  3  4
	//    abs: -2 -1 +0 +1 +2
	// counts:  1  0  0  2  1
}
