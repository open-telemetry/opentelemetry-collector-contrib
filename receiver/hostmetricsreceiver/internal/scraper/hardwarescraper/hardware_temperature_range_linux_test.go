// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package hardwarescraper

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Readings are published as read: the scraper does not decide which
// temperatures are plausible. Industrial and cryogenic sensors legitimately
// report values outside any hard-coded range.
func TestRange_ReadingsOutsideAnyPlausibleRangeAreEmitted(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Test is for Linux platform")
	}

	base := t.TempDir()
	writeSensor(t, base, "hwmon0", "cryochip", "temp1", "-60000", "Cold", "", "")
	writeSensor(t, base, "hwmon1", "furnacechip", "temp1", "250000", "Hot", "", "")

	_, pts := collect(t, scrapeFixture(t, base, false), "hw.temperature")
	require.Len(t, pts, 2)

	byLocation := map[string]float64{}
	for _, p := range pts {
		byLocation[p.attr["hw.sensor_location"].(string)] = p.val
	}
	assert.InDelta(t, -60.0, byLocation["Cold"], 0.001, "a reading below -40 C is emitted as read")
	assert.InDelta(t, 250.0, byLocation["Hot"], 0.001, "a reading above 200 C is emitted as read")
}

// Thresholds get the same treatment as readings.
func TestRange_ThresholdsOutsideAnyPlausibleRangeAreEmitted(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Test is for Linux platform")
	}

	base := t.TempDir()
	writeSensor(t, base, "hwmon0", "furnacechip", "temp1", "150000", "Hot", "260000", "240000")

	_, pts := collect(t, scrapeFixture(t, base, true), "hw.temperature.limit")
	require.Len(t, pts, 2, "both thresholds are emitted although they exceed 200 C")

	byLimit := map[string]float64{}
	for _, p := range pts {
		byLimit[p.attr["hw.limit_type"].(string)] = p.val
	}
	assert.InDelta(t, 260.0, byLimit["high.critical"], 0.001)
	assert.InDelta(t, 240.0, byLimit["high.degraded"], 0.001)
}

// Documented limitation: a driver may expose sensors it never populates, and
// those read as a successful 0. There is no runtime signal separating them from
// a genuine 0 C measurement, so they are published as 0 C.
func TestRange_UnpopulatedSensorIsPublishedAsZero(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Test is for Linux platform")
	}

	base := t.TempDir()
	writeSensor(t, base, "hwmon0", "thinkpad", "temp1", "66000", "CPU", "", "")
	// Same layout as an unpopulated sensor on a real thinkpad: readable, zero,
	// no label and no thresholds.
	writeSensor(t, base, "hwmon0", "thinkpad", "temp3", "0", "", "", "")

	_, pts := collect(t, scrapeFixture(t, base, false), "hw.temperature")
	require.Len(t, pts, 2)

	byLocation := map[string]float64{}
	for _, p := range pts {
		byLocation[p.attr["hw.sensor_location"].(string)] = p.val
	}
	assert.InDelta(t, 66.0, byLocation["CPU"], 0.001)
	assert.InDelta(t, 0.0, byLocation["temp3"], 0.001, "unpopulated sensors are reported as 0 C, as documented")
}
