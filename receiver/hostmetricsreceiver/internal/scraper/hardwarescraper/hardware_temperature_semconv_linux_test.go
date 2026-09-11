// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package hardwarescraper

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/scraper/scrapertest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/hardwarescraper/internal/metadata"
)

// writeSensor lays out a single hwmon chip: name, temperature input and
// optionally a label and thresholds. An empty string means "no such file".
func writeSensor(t *testing.T, base, dir, chipName, sensor, input, label, crit, maxv string) {
	t.Helper()
	d := filepath.Join(base, dir)
	require.NoError(t, os.MkdirAll(d, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(d, "name"), []byte(chipName), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(d, sensor+"_input"), []byte(input), 0o600))
	for suffix, val := range map[string]string{"_label": label, "_crit": crit, "_max": maxv} {
		if val != "" {
			require.NoError(t, os.WriteFile(filepath.Join(d, sensor+suffix), []byte(val), 0o600))
		}
	}
}

// realHardwareFixture reproduces a sensor layout observed on an industrial PC:
// coretemp with labels and thresholds, acpitz without a label and without
// thresholds, spd5118 with thresholds but without a label.
func realHardwareFixture(t *testing.T) string {
	t.Helper()
	base := t.TempDir()
	writeSensor(t, base, "hwmon0", "acpitz", "temp1", "51000", "", "", "")
	writeSensor(t, base, "hwmon1", "spd5118", "temp1", "52500", "", "85000", "55000")
	writeSensor(t, base, "hwmon2", "coretemp", "temp1", "47000", "Package id 0", "100000", "80000")
	writeSensor(t, base, "hwmon2", "coretemp", "temp2", "48000", "Core 0", "100000", "80000")
	return base
}

func scrapeFixture(t *testing.T, hwmonPath string, withLimits bool) pmetric.Metrics {
	t.Helper()
	cfg := metadata.NewDefaultMetricsBuilderConfig()
	cfg.Metrics.HwTemperature.Enabled = true
	cfg.Metrics.HwTemperatureLimit.Enabled = withLimits

	s := &hardwareTemperatureScraper{
		logger:               zap.NewNop(),
		config:               &TemperatureConfig{},
		hwmonPath:            hwmonPath,
		metricsBuilderConfig: cfg,
	}
	require.NoError(t, s.start(t.Context()))

	mb := metadata.NewMetricsBuilder(cfg, scrapertest.NewNopSettings(metadata.Type))
	require.NoError(t, s.scrape(t.Context(), mb))
	return mb.Emit()
}

// collect gathers the data points of the given metric (value plus attributes).
func collect(t *testing.T, m pmetric.Metrics, name string) (unit string, pts []struct {
	val  float64
	attr map[string]any
},
) {
	t.Helper()
	rms := m.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		sms := rms.At(i).ScopeMetrics()
		for j := 0; j < sms.Len(); j++ {
			ms := sms.At(j).Metrics()
			for k := 0; k < ms.Len(); k++ {
				metric := ms.At(k)
				if metric.Name() != name {
					continue
				}
				unit = metric.Unit()
				require.Equal(t, pmetric.MetricTypeGauge, metric.Type(), "%s must be a gauge", name)
				dps := metric.Gauge().DataPoints()
				for d := 0; d < dps.Len(); d++ {
					pts = append(pts, struct {
						val  float64
						attr map[string]any
					}{dps.At(d).DoubleValue(), dps.At(d).Attributes().AsRaw()})
				}
			}
		}
	}
	return unit, pts
}

// The core test: metric names, unit and ATTRIBUTE KEYS follow the OTel hardware
// semantic conventions (hw. prefix).
func TestSemconv_TemperatureNamesUnitsAndAttributeKeys(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Test is for Linux platform")
	}

	unit, pts := collect(t, scrapeFixture(t, realHardwareFixture(t), false), "hw.temperature")
	require.NotEmpty(t, pts, "hw.temperature must be emitted")
	assert.Equal(t, "Cel", unit, "the unit must be UCUM Cel")

	for _, p := range pts {
		// Convention: hw.id is required; hw.name, hw.parent and
		// hw.sensor_location are recommended.
		assert.Contains(t, p.attr, "hw.id")
		assert.Contains(t, p.attr, "hw.name")
		assert.Contains(t, p.attr, "hw.parent")
		assert.Contains(t, p.attr, "hw.sensor_location")
		// The convention does not put hw.type on hw.temperature; it is required
		// on hw.status only.
		assert.NotContains(t, p.attr, "hw.type")
		// Unprefixed attributes are a direct violation of the convention.
		for _, bad := range []string{"id", "name", "parent", "sensor_location", "limit_type"} {
			assert.NotContains(t, p.attr, bad, "attribute %q must carry the hw. prefix", bad)
		}
	}
}

// Millidegree to degree conversion, and the values of hw.name and
// hw.sensor_location on a realistic sensor layout.
func TestSemconv_AttributeValuesOnRealHardware(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Test is for Linux platform")
	}

	_, pts := collect(t, scrapeFixture(t, realHardwareFixture(t), false), "hw.temperature")
	require.Len(t, pts, 4, "four sensors: acpitz, spd5118 and coretemp twice")

	// Index by hw.id: it is the only unique attribute. hw.sensor_location is not
	// an identifier -- acpitz and spd5118 carry no label, so both report "temp1".
	byID := map[string]struct {
		val  float64
		attr map[string]any
	}{}
	for _, p := range pts {
		byID[p.attr["hw.id"].(string)] = p
	}

	// The fixture has no `device` symlink, so deviceKey is the hwmonN directory name.
	cases := []struct {
		id, name, location string
		val                float64
	}{
		// coretemp publishes labels, so the location is the hardware label.
		{"coretemp_hwmon2_temp1", "coretemp", "Package id 0", 47.0},
		{"coretemp_hwmon2_temp2", "coretemp", "Core 0", 48.0},
		// No label, so the location falls back to the sysfs designation.
		{"acpitz_hwmon0_temp1", "acpitz", "temp1", 51.0},
		{"spd5118_hwmon1_temp1", "spd5118", "temp1", 52.5},
	}
	for _, c := range cases {
		p, ok := byID[c.id]
		require.True(t, ok, "expected a sensor with hw.id=%q, got: %v", c.id, keys(byID))
		assert.InDelta(t, c.val, p.val, 0.001, "%s: millidegrees must be converted to degrees", c.id)
		assert.Equal(t, c.name, p.attr["hw.name"], "%s: hw.name is the chip name, not the sensor label", c.id)
		assert.Equal(t, c.location, p.attr["hw.sensor_location"], "%s: location is the label or the sysfs designation", c.id)
		assert.Contains(t, p.attr["hw.parent"], c.name, "%s: hw.parent identifies the chip", c.id)
	}

	// Synthetic locations such as CORETEMP_..._TEMP1 must not appear.
	for _, p := range pts {
		assert.NotContains(t, p.attr["hw.sensor_location"], "_TEMP",
			"location must not be synthesized: %v", p.attr["hw.sensor_location"])
	}
}

func keys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// hw.id is unique within the host, including chips that share a name.
func TestSemconv_HwIDUniqueAcrossChipsWithSameName(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Test is for Linux platform")
	}

	base := t.TempDir()
	// Two distinct devices with the SAME name, the typical case being several NVMe drives.
	writeSensor(t, base, "hwmon0", "nvme", "temp1", "40000", "", "", "")
	writeSensor(t, base, "hwmon1", "nvme", "temp1", "41000", "", "", "")

	_, pts := collect(t, scrapeFixture(t, base, false), "hw.temperature")
	require.Len(t, pts, 2)

	ids := map[string]bool{}
	for _, p := range pts {
		ids[p.attr["hw.id"].(string)] = true
	}
	assert.Len(t, ids, 2, "chips sharing a name must get distinct hw.id, otherwise the series collapse")
}

// Thresholds: crit maps to high.critical, max maps to high.degraded, and a
// sensor without thresholds emits no limit metric at all.
func TestSemconv_TemperatureLimits(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Test is for Linux platform")
	}

	unit, pts := collect(t, scrapeFixture(t, realHardwareFixture(t), true), "hw.temperature.limit")
	require.NotEmpty(t, pts)
	assert.Equal(t, "Cel", unit)

	// Key on hw.id, which is unique, plus the limit type; location does not work here.
	type key struct{ id, limit string }
	got := map[key]float64{}
	for _, p := range pts {
		assert.Contains(t, p.attr, "hw.limit_type")
		got[key{p.attr["hw.id"].(string), p.attr["hw.limit_type"].(string)}] = p.val
	}

	// coretemp: crit=100000, max=80000.
	assert.InDelta(t, 100.0, got[key{"coretemp_hwmon2_temp1", "high.critical"}], 0.001, "temp*_crit -> high.critical")
	assert.InDelta(t, 80.0, got[key{"coretemp_hwmon2_temp1", "high.degraded"}], 0.001, "temp*_max -> high.degraded")
	// spd5118: crit=85000, max=55000 -- thresholds present, label absent.
	assert.InDelta(t, 85.0, got[key{"spd5118_hwmon1_temp1", "high.critical"}], 0.001)
	assert.InDelta(t, 55.0, got[key{"spd5118_hwmon1_temp1", "high.degraded"}], 0.001)

	// acpitz publishes no thresholds, so it must not produce any limit point.
	for k := range got {
		assert.NotEqual(t, "acpitz_hwmon0_temp1", k.id,
			"a sensor without thresholds (acpitz) must not report hw.temperature.limit")
	}
}

// A missing hwmon directory must not turn into an error.
func TestSemconv_NoHwmonNoError(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Test is for Linux platform")
	}

	_, pts := collect(t, scrapeFixture(t, filepath.Join(t.TempDir(), "nonexistent"), false), "hw.temperature")
	assert.Empty(t, pts, "no sensors means no metrics, but it must not fail either")
}
