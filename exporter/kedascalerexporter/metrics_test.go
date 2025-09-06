// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kedascalerexporter

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/exporter/exportertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kedascalerexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
)

func TestMetrics(t *testing.T) {
	tests := []struct {
		name           string
		testdata       string
		query          string
		expectedResult float64
	}{
		{
			name:           "sum and rate http_server_duration_bucket",
			testdata:       "testdata/input.yaml",
			query:          `histogram_quantile(0.95, sum(rate(http_server_duration_bucket[1m])) by (le))`,
			expectedResult: 80.00000000000001,
		},
		{
			name:           "avg first monotonic sum",
			testdata:       "testdata/input.yaml",
			query:          `avg(first_monotonic_sum{aaa="bbb"})`,
			expectedResult: 327,
		},
		{
			name:           "min first monotonic",
			testdata:       "testdata/input.yaml",
			query:          `min(first_monotonic_sum{aaa="bbb"})`,
			expectedResult: 321,
		},
		{
			name:           "max first monotonic",
			testdata:       "testdata/input.yaml",
			query:          `max(first_monotonic_sum{aaa="bbb"})`,
			expectedResult: 333,
		},
		{
			name:           "sum first monotonic sum resource_key",
			testdata:       "testdata/input.yaml",
			query:          `sum(first_monotonic_sum{aaa="bbb", job="foo"})`,
			expectedResult: 333,
		},
		{
			name:           "sum with ADD",
			testdata:       "testdata/input.yaml",
			query:          `sum(first_monotonic_sum) + sum(second_monotonic_sum)`,
			expectedResult: 1722,
		},
		{
			name:           "sum with MUL",
			testdata:       "testdata/input.yaml",
			query:          `sum(first_monotonic_sum) * sum(second_monotonic_sum)`,
			expectedResult: 698472,
		},
		{
			name:           "sum with first_monotonic",
			testdata:       "testdata/input.yaml",
			query:          `sum(first_monotonic_sum)`,
			expectedResult: 654,
		},
		{
			name:           "sum with second_monotonic",
			testdata:       "testdata/input.yaml",
			query:          `sum(second_monotonic_sum)`,
			expectedResult: 1068,
		},
		{
			name:           "sum with SPLIT",
			testdata:       "testdata/input.yaml",
			query:          `sum(first_monotonic_sum) / sum(second_monotonic_sum)`,
			expectedResult: 0.6123595505617978,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input, err := golden.ReadMetrics(tt.testdata)
			require.NoError(t, err)
			defaultConfig := CreateDefaultConfig()
			exporter, err := newKedaScalerExporter(
				t.Context(),
				defaultConfig.(*Config),
				exportertest.NewNopSettings(metadata.Type),
			)
			require.NoError(t, err)
			defer exporter.metricStore.Close()

			err = exporter.ConsumeMetrics(t.Context(), input)
			require.NoError(t, err)

			// Use the timestamp from the test data (1746036255000000000 nanoseconds)
			start := int64(1746036255000) // Convert to milliseconds
			result, err := exporter.metricStore.Query(tt.query, time.UnixMilli(start))
			require.NoError(t, err)
			require.True(
				t,
				equalInFirstTwoDecimals(tt.expectedResult, result),
				"expected %f, got %f",
				tt.expectedResult,
				result,
			)
		})
	}
}

func equalInFirstTwoDecimals(a, b float64) bool {
	// Check if integer parts are equal
	if int(a) != int(b) {
		return false
	}

	// Extract first two decimal places
	aScaled := int(a*100) % 100
	bScaled := int(b*100) % 100

	return aScaled == bScaled
}
