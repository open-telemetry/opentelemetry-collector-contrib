// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuremonitorreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azuremonitorreceiver"

var unitMapping = map[string]string{
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
	"Unspecified":    "", // Empty string signals "no unit" in OpenTelemetry, even though it's not a valid UCUM unit.
}

// convertUnitToUCUM converts a given unit to its corresponding UCUM representation. If
// the unit is not found in the mapping, it returns the unit as-is.
func convertUnitToUCUM(unit string) string {
	if val, ok := unitMapping[unit]; ok {
		return val
	}
	return unit
}
