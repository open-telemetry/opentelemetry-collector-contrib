// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/metrics"

// RemoveIfFloat64 removes elements from a float64 slice based on a predicate function.
// The slice is modified in-place and the new slice is returned.
func RemoveIfFloat64(slice []float64, predicate func(float64, int) bool) []float64 {
	n := 0
	for i := 0; i < len(slice); i++ {
		if !predicate(slice[i], i) {
			slice[n] = slice[i]
			n++
		}
	}
	return slice[:n]
}

// RemoveIfUint64 removes elements from a uint64 slice based on a predicate function.
// The slice is modified in-place and the new slice is returned.
// The predicate function receives both the value and its index in the original slice.
func RemoveIfUint64(slice []uint64, predicate func(uint64, int) bool) []uint64 {
	n := 0
	for i := 0; i < len(slice); i++ {
		if !predicate(slice[i], i) {
			slice[n] = slice[i]
			n++
		}
	}
	return slice[:n]
}
