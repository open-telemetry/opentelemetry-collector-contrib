// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package precision

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRatioUint64(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name                    string
		numerator, denominator  uint64
		expected                float64
	}{
		{
			name:        "issue example 32GB machine",
			numerator:   21208754,
			denominator: 33554432,
			expected:    0.63207012,
		},
		{
			name:        "issue example 1TB machine",
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
			actual := RatioUint64(tt.numerator, tt.denominator)
			assert.Equal(t, tt.expected, actual)
		})
	}
}

func TestRatioUint64_ZeroDenominator(t *testing.T) {
	t.Parallel()
	assert.True(t, math.IsInf(RatioUint64(100, 0), 1))
	assert.True(t, math.IsNaN(RatioUint64(0, 0)))
}

func TestScaleUint64(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name                    string
		numerator, denominator  uint64
		expected                float64
	}{
		{
			name:        "milliseconds to seconds",
			numerator:   12345,
			denominator: 1000,
			expected:    12.345,
		},
		{
			name:        "jiffies USER_HZ=100",
			numerator:   12345,
			denominator: 100,
			expected:    123.45,
		},
		{
			name:        "zero numerator",
			numerator:   0,
			denominator: 1000,
			expected:    0.0,
		},
		{
			name:        "exact division",
			numerator:   3000,
			denominator: 1000,
			expected:    3.0,
		},
		{
			name:        "windows counter units 1e7",
			numerator:   15000000,
			denominator: 10000000,
			expected:    1.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			actual := ScaleUint64(tt.numerator, tt.denominator)
			assert.Equal(t, tt.expected, actual)
		})
	}
}

func TestScaleUint64_ZeroDenominator(t *testing.T) {
	t.Parallel()
	assert.True(t, math.IsInf(ScaleUint64(100, 0), 1))
	assert.True(t, math.IsNaN(ScaleUint64(0, 0)))
}

func TestRatioUint64_PrecisionScalesWithMagnitude(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name                    string
		numerator, denominator  uint64
		expected                float64
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
			actual := RatioUint64(tt.numerator, tt.denominator)
			assert.Equal(t, tt.expected, actual)
		})
	}
}
