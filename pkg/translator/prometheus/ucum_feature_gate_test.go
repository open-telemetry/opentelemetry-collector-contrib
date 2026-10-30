// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheus

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/featuregate"
)

func TestCorrectUCUMUnitsFeatureGate(t *testing.T) {
	const gateID = "pkg.prometheus.CorrectUCUMUnits"
	registry := featuregate.GlobalRegistry()

	require.NoError(t, registry.Set(gateID, false))
	t.Cleanup(func() {
		require.NoError(t, registry.Set(gateID, false))
	})

	tests := []struct {
		name      string
		input     string
		legacy    string
		corrected string
		convert   func(string) string
	}{
		{
			"minute to UCUM",
			"requests_per_minute",
			"requests/m",
			"requests/min",
			UnitWordToUCUM,
		},
		{
			"week to UCUM",
			"requests_per_week",
			"requests/w",
			"requests/wk",
			UnitWordToUCUM,
		},
		{
			"year to UCUM",
			"requests_per_year",
			"requests/y",
			"requests/a",
			UnitWordToUCUM,
		},
		{
			"tebibytes to UCUM",
			"tebibytes",
			"tebibytes",
			"TiBy",
			UnitWordToUCUM,
		},
		{
			"kilobytes to UCUM",
			"kilobytes",
			"KBy",
			"kBy",
			UnitWordToUCUM,
		},
		{
			"minute to Prometheus",
			"By/min",
			"bytes_per_min",
			"bytes_per_minute",
			BuildCompliantPrometheusUnit,
		},
		{
			"meter must not become minute",
			"By/m",
			"bytes_per_minute",
			"bytes_per_m",
			BuildCompliantPrometheusUnit,
		},
		{
			"week to Prometheus",
			"By/wk",
			"bytes_per_wk",
			"bytes_per_week",
			BuildCompliantPrometheusUnit,
		},
		{
			"year to Prometheus",
			"By/a",
			"bytes_per_a",
			"bytes_per_year",
			BuildCompliantPrometheusUnit,
		},
		{
			"tebibytes to Prometheus",
			"TiBy",
			"tibibytes",
			"tebibytes",
			BuildCompliantPrometheusUnit,
		},
		{
			"kilobytes to Prometheus",
			"kBy",
			"kBy",
			"kilobytes",
			BuildCompliantPrometheusUnit,
		},
	}

	for _, enabled := range []bool{false, true} {
		require.NoError(t, registry.Set(gateID, enabled))

		for _, tc := range tests {
			expected := tc.legacy
			if enabled {
				expected = tc.corrected
			}

			require.Equal(
				t,
				expected,
				tc.convert(tc.input),
				"%s (gate enabled: %t)",
				tc.name,
				enabled,
			)
		}
	}
}
