// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheus // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus"

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/featuregate"
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
		{
			input:    "requests_per_minute",
			expected: "requests/m",
		},
		{
			input:    "requests_per_week",
			expected: "requests/w",
		},
		{
			input:    "requests_per_year",
			expected: "requests/y",
		},
		{
			input:    "tibibytes",
			expected: "TiBy",
		},
		{
			input:    "tebibytes",
			expected: "tebibytes",
		},
		{
			input:    "kilobytes",
			expected: "KBy",
		},
	} {
		t.Run(fmt.Sprintf("input: \"%v\"", tc.input), func(t *testing.T) {
			got := UnitWordToUCUM(tc.input)
			assert.Equal(t, tc.expected, got)
		})
	}
}

func TestUnitWordToUCUM_SpecCompliant(t *testing.T) {
	require.NoError(t, featuregate.GlobalRegistry().Set(SpecCompliantUCUMUnitsGate.ID(), true))
	defer func() {
		require.NoError(t, featuregate.GlobalRegistry().Set(SpecCompliantUCUMUnitsGate.ID(), false))
	}()

	for _, tc := range []struct {
		input    string
		expected string
	}{
		{
			input:    "requests_per_minute",
			expected: "requests/min",
		},
		{
			input:    "requests_per_week",
			expected: "requests/wk",
		},
		{
			input:    "requests_per_year",
			expected: "requests/a",
		},
		{
			input:    "tebibytes",
			expected: "TiBy",
		},
		{
			input:    "tibibytes",
			expected: "TiBy",
		},
		{
			input:    "kilobytes",
			expected: "kBy",
		},
	} {
		t.Run(fmt.Sprintf("input: \"%v\"", tc.input), func(t *testing.T) {
			got := UnitWordToUCUM(tc.input)
			assert.Equal(t, tc.expected, got)
		})
	}
}
