// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package expotest

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data/expo"
)

var t testing.TB = fakeT{}

var expotest = struct {
	Is      func(t testing.TB) T
	Observe func(expo.Scale, ...float64) expo.Buckets
}{
	Is:      Is,
	Observe: Observe,
}

func ExampleT_Equal() {
	is := expotest.Is(t)

	want := Histogram{
		PosNeg: expotest.Observe(expo.Scale(0), 1, 2, 3, 4),
		Scale:  0,
	}.Into()

	got := Histogram{
		PosNeg: expotest.Observe(expo.Scale(1), 1, 1, 1, 1),
		Scale:  1,
	}.Into()

	is.Equal(want, got)

	// Output:
	// equal_test.go:40: Negative().BucketCounts().AsRaw(): [1 1 2] != [4]
	// equal_test.go:40: Negative().BucketCounts().Len(): 3 != 1
	// equal_test.go:40: Positive().BucketCounts().AsRaw(): [1 1 2] != [4]
	// equal_test.go:40: Positive().BucketCounts().Len(): 3 != 1
	// equal_test.go:40: Scale(): 0 != 1
}

func TestNone(*testing.T) {}

type fakeT struct {
	testing.TB
}

func (t fakeT) Helper() {}

func (t fakeT) Errorf(format string, args ...any) {
	var from string
	for i := 0; ; i++ {
		pc, file, line, ok := runtime.Caller(i)
		if !ok {
			break
		}
		fn := runtime.FuncForPC(pc)
		if strings.HasSuffix(fn.Name(), ".ExampleT_Equal") {
			from = filepath.Base(file) + ":" + strconv.Itoa(line)
			break
		}
	}

	fmt.Printf("%s: %s\n", from, fmt.Sprintf(format, args...))
}
