// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheus // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus"

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUnitWordToUCUM(t *testing.T) {
	for _, tc := range []struct {
		input    string
		expected string
	}{
		{
			input:    "",
			expected: "",
		},
		{
			input:    "days",
			expected: "d",
		},
		{
			input:    "seconds",
			expected: "s",
		},
		{
			input:    "kibibytes",
			expected: "KiBy",
		},
		{
			input:    "volts",
			expected: "V",
		},
		{
			input:    "bananas_per_day",
			expected: "bananas/d",
		},
		{
			input:    "meters_per_hour",
			expected: "m/h",
		},
		{
			input:    "ratio",
			expected: "1",
		},
		{
			input:    "percent",
			expected: "%",
		},
	} {
		t.Run(fmt.Sprintf("input: \"%v\"", tc.input), func(t *testing.T) {
			got := UnitWordToUCUM(tc.input)
			assert.Equal(t, tc.expected, got)
		})
	}

}
