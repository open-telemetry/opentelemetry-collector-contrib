// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package precision

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRatio(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name                   string
		numerator, denominator uint64
		expected               float64
	}{
		{
			name:        "memory usage 32GB machine",
			numerator:   21208754,
			denominator: 33554432,
			expected:    0.63207012,
		},
		{
			name:        "memory usage 1TB machine",
			numerator:   687397760,
			denominator: 1073741824,
			expected:    0.6401890516,
		},
		{
			name:        "small magnitude",
			numerator:   1,
			denominator: 3,
			expected:    0.3,
		},
		{
			name:        "zero numerator",
			numerator:   0,
			denominator: 100,
			expected:    0.0,
		},
		{
			name:        "power of 10 numerator",
			numerator:   10,
			denominator: 3,
			expected:    3.33,
		},
		{
			name:        "power of 10 both",
			numerator:   100,
			denominator: 10,
			expected:    10.0,
		},
		{
			name:        "equal values",
			numerator:   100,
			denominator: 100,
			expected:    1.0,
		},
		{
			name:        "large equal values",
			numerator:   33554432,
			denominator: 33554432,
			expected:    1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			actual := Ratio(tt.numerator, tt.denominator)
			assert.Equal(t, tt.expected, actual)
		})
	}
}

func TestRatio_ZeroDenominator(t *testing.T) {
	t.Parallel()
	assert.True(t, math.IsInf(Ratio(100, 0), 1))
	assert.True(t, math.IsNaN(Ratio(0, 0)))
}

func TestScale(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    uint64
		unit     time.Duration
		expected float64
	}{
		{name: "milliseconds standard", input: 12345, unit: time.Millisecond, expected: 12.345},
		{name: "milliseconds zero", input: 0, unit: time.Millisecond, expected: 0.0},
		{name: "milliseconds exact", input: 3000, unit: time.Millisecond, expected: 3.0},
		{name: "milliseconds sub-second", input: 999, unit: time.Millisecond, expected: 0.999},
		{name: "milliseconds one", input: 1, unit: time.Millisecond, expected: 0.001},
		{name: "nanoseconds standard", input: 1_500_000_000, unit: time.Nanosecond, expected: 1.5},
		{name: "nanoseconds zero", input: 0, unit: time.Nanosecond, expected: 0.0},
		{name: "nanoseconds large", input: 3_000_000_000, unit: time.Nanosecond, expected: 3.0},
		{name: "nanoseconds sub-millisecond", input: 999_999, unit: time.Nanosecond, expected: 0.000999999},
		{name: "nanoseconds sub-microsecond", input: 999, unit: time.Nanosecond, expected: 0.000000999},
		{name: "nanoseconds one", input: 1, unit: time.Nanosecond, expected: 0.000000001},
		{name: "100-nanoseconds standard", input: 15_000_000, unit: time.Nanosecond * 100, expected: 1.5},
		{name: "100-nanoseconds zero", input: 0, unit: time.Nanosecond * 100, expected: 0.0},
		{name: "100-nanoseconds exact", input: 10_000_000, unit: time.Nanosecond * 100, expected: 1.0},
		{name: "100-nanoseconds sub-millisecond", input: 9_999, unit: time.Nanosecond * 100, expected: 0.0009999},
		{name: "100-nanoseconds sub-microsecond", input: 9, unit: time.Nanosecond * 100, expected: 0.0000009},
		{name: "100-nanoseconds one", input: 1, unit: time.Nanosecond * 100, expected: 0.0000001},
		{name: "microseconds", input: 12345, unit: time.Microsecond, expected: 0.012345},
		{name: "10ms jiffies standard", input: 12345, unit: time.Millisecond * 10, expected: 123.45},
		{name: "10ms jiffies exact", input: 300, unit: time.Millisecond * 10, expected: 3.0},
		{name: "10ms jiffies zero", input: 0, unit: time.Millisecond * 10, expected: 0.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, Scale(tt.input, tt.unit))
		})
	}
}

func TestRatio_PrecisionScalesWithMagnitude(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name                   string
		numerator, denominator uint64
		expected               float64
	}{
		{name: "magnitude 1e1", numerator: 1, denominator: 3, expected: 0.3},
		{name: "magnitude 1e2", numerator: 10, denominator: 30, expected: 0.33},
		{name: "magnitude 1e3", numerator: 100, denominator: 300, expected: 0.333},
		{name: "magnitude 1e4", numerator: 1000, denominator: 3000, expected: 0.3333},
		{name: "magnitude 1e5", numerator: 10000, denominator: 30000, expected: 0.33333},
		{name: "magnitude 1e6", numerator: 100000, denominator: 300000, expected: 0.333333},
		{name: "magnitude 1e7", numerator: 1000000, denominator: 3000000, expected: 0.3333333},
		{name: "magnitude 1e8", numerator: 10000000, denominator: 30000000, expected: 0.33333333},
		{name: "magnitude 1e9", numerator: 100000000, denominator: 300000000, expected: 0.333333333},
		{name: "magnitude 1e10", numerator: 1000000000, denominator: 3000000000, expected: 0.3333333333},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			actual := Ratio(tt.numerator, tt.denominator)
			assert.Equal(t, tt.expected, actual)
		})
	}
}

var benchSink float64

var ratioInputs = [][2]uint64{
	{21208754, 33554432},
	{687397760, 1073741824},
	{1, 3},
	{100, 100},
	{8589934592, 17179869184},
}

var scaleInputs = []uint64{
	12345,
	999,
	15000000,
	3000000000,
	50000,
}

func BenchmarkRatio(b *testing.B) {
	for i := range b.N {
		benchSink = Ratio(ratioInputs[i%len(ratioInputs)][0], ratioInputs[i%len(ratioInputs)][1])
	}
}

func BenchmarkRatio_RawDivision(b *testing.B) {
	for i := range b.N {
		benchSink = float64(ratioInputs[i%len(ratioInputs)][0]) / float64(ratioInputs[i%len(ratioInputs)][1])
	}
}

func BenchmarkScale(b *testing.B) {
	for i := range b.N {
		benchSink = Scale(scaleInputs[i%len(scaleInputs)], time.Millisecond)
	}
}

func BenchmarkScale_RawDivision(b *testing.B) {
	for i := range b.N {
		benchSink = float64(scaleInputs[i%len(scaleInputs)]) / 1000.0
	}
}
