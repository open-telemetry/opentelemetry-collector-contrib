// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pslice // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/putil/pslice"

type Slice[E any] interface {
	At(int) E
	Len() int
}

func Equal[E comparable, S Slice[E]](a, b S) bool {
	if a.Len() != b.Len() {
		return false
	}
	for i := range a.Len() {
		if a.At(i) != b.At(i) {
			return false
		}
	}
	return true
}

func All[E any, S Slice[E]](slice S) func(func(E) bool) {
	return func(yield func(E) bool) {
		for i := range slice.Len() {
			if !yield(slice.At(i)) {
				break
			}
		}
	}
}
