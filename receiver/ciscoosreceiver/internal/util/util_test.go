// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package util

import "testing"

func TestStr2float64(t *testing.T) {
	tests := []struct {
		input    string
		expected float64
	}{
		{"-", 0.0},
		{"0", 0.0},
		{"123", 123.0},
		{"1000000", 1000000.0},
		{"invalid", 0.0},
		{"", 0.0},
	}

	for _, test := range tests {
		result := Str2float64(test.input)
		if result != test.expected {
			t.Errorf("Str2float64(%q) = %f, expected %f", test.input, result, test.expected)
		}
	}
}
