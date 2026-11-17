// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuremonitorreceiver

import (
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/monitor/query/azmetrics"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/monitor/armmonitor"
	"github.com/iimos/ucum"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertUnitToUCUM(t *testing.T) {
	tests := map[string]string{
		"Count":          "{count}",
		"CountPerSecond": "{count}/s",
		"Bytes":          "By",
		"BytesPerSecond": "By/s",
		"ByteSeconds":    "By.s",
		"BitsPerSecond":  "bit/s",
		"Percent":        "%",
		"Seconds":        "s",
		"MilliSeconds":   "ms",
		"Cores":          "{cpu}",
		"MilliCores":     "10^-3{cpu}",
		"NanoCores":      "10^-9{cpu}",
		"Unspecified":    "",
		"SomethingNew":   "SomethingNew",
	}
	for in, want := range tests {
		assert.Equal(t, want, convertUnitToUCUM(in), in)
	}
}

func TestARMAPIUnitCoverage(t *testing.T) {
	for _, unit := range armmonitor.PossibleMetricUnitValues() {
		strUnit := string(unit)
		t.Run(strUnit, func(t *testing.T) {
			_, ok := unitMapping[strUnit]
			require.True(t, ok, "unit %q not found in unitMapping", strUnit)
		})
	}
}

func TestBatchAPIUnitCoverage(t *testing.T) {
	for _, unit := range azmetrics.PossibleMetricUnitValues() {
		strUnit := string(unit)
		t.Run(strUnit, func(t *testing.T) {
			_, ok := unitMapping[strUnit]
			require.True(t, ok, "unit %q not found in unitMapping", strUnit)
		})
	}
}

func TestAllMappedValuesAreValidUCUM(t *testing.T) {
	for unit := range unitMapping {
		t.Run(unit, func(t *testing.T) {
			got := convertUnitToUCUM(unit)
			if unit != "Unspecified" { // Unspecified intentionally returns empty string
				_, err := ucum.Parse([]byte(got))
				require.NoError(t, err, "convertUnitToUCUM() = %q, an invalid UCUM string", got)
			}
		})
	}
}
