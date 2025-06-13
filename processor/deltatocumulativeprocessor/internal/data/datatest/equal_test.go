// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datatest

import (
	"fmt"
	"iter"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data/expo"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data/expo/expotest"
)

var tb testing.TB = fakeT{}

var datatest = struct{ New func(tb testing.TB) T }{New: New}

func ExampleT_Equal() {
	is := datatest.New(tb)

	want := expotest.Histogram{
		PosNeg: expotest.Observe(expo.Scale(0), 1, 2, 3, 4),
		Scale:  0,
	}.Into()

	got := expotest.Histogram{
		PosNeg: expotest.Observe(expo.Scale(1), 1, 1, 1, 1),
		Scale:  1,
	}.Into()

	is.Equal(want, got)

	// Output:
	// equal_test.go:36: Negative().BucketCounts().AsRaw(): [1 1 2] != [4]
	// equal_test.go:36: Negative().BucketCounts().Len(): 3 != 1
	// equal_test.go:36: Positive().BucketCounts().AsRaw(): [1 1 2] != [4]
	// equal_test.go:36: Positive().BucketCounts().Len(): 3 != 1
	// equal_test.go:36: Scale(): 0 != 1
}

type structFunc struct {
	a int
}

func (s structFunc) Get() int {
	return s.a
}

func (s structFunc) Func() func() {
	return func() {}
}

// iter.Seq is a reflect.Func
func (s structFunc) Seq() iter.Seq[int] {
	return func(_ func(v int) bool) {
	}
}

// iter.Seq2 is a reflect.Func
func (s structFunc) Seq2() iter.Seq2[int, string] {
	return func(_ func(k int, v string) bool) {
	}
}

func TestEqualMethodIgnoreFuncReturnType(t *testing.T) {
	is := datatest.New(t)
	s := structFunc{a: 42}
	want := any(s)
	got := any(s)
	is.Equal(want, got)
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
