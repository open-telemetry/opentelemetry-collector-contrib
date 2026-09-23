// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package hardwarescraper

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/scraper/scrapererror"
	"go.opentelemetry.io/collector/scraper/scrapertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/hardwarescraper/internal/metadata"
)

// The failure count reported to the collector must reflect how many sensors
// actually failed, not a constant chosen by the wrapper.
func TestPartialError_CountsEveryFailedSensor(t *testing.T) {
	base := t.TempDir()
	writeSensor(t, base, "hwmon0", "coretemp", "temp1", "47000", "Package id 0", "", "")
	// Two sensors whose input does not parse: both must be counted.
	writeSensor(t, base, "hwmon1", "brokenchip", "temp1", "not-a-number", "", "", "")
	writeSensor(t, base, "hwmon2", "brokenchip", "temp1", "also-broken", "", "", "")

	cfg := metadata.NewDefaultMetricsBuilderConfig()
	cfg.Metrics.HwTemperature.Enabled = true

	s := newHardwareScraper(t.Context(), scrapertest.NewNopSettings(metadata.Type), &Config{
		MetricsBuilderConfig: cfg,
		HwmonPath:            base,
		Temperature:          &TemperatureConfig{},
	})
	require.NoError(t, s.start(t.Context(), nil))

	m, err := s.scrape(t.Context())
	require.Error(t, err)

	var partialErr scrapererror.PartialScrapeError
	require.ErrorAs(t, err, &partialErr, "a failed sensor must surface as a partial scrape error")
	assert.Equal(t, 2, partialErr.Failed,
		"each failed temperature input must contribute one failed metric")

	_, pts := collect(t, m, "hw.temperature")
	assert.Len(t, pts, 1, "the healthy sensor is still reported")
}

// a failed temperature input does not prevent thresholds from the same sensor
// from being reported, so it contributes only one failed metric.
func TestPartialError_PreservesLimitsWhenTemperatureInputFails(t *testing.T) {
	base := t.TempDir()
	writeSensor(t, base, "hwmon0", "coretemp", "temp1", "not-a-number", "CPU", "100000", "80000")

	cfg := metadata.NewDefaultMetricsBuilderConfig()
	cfg.Metrics.HwTemperatureLimit.Enabled = true

	s := newHardwareScraper(t.Context(), scrapertest.NewNopSettings(metadata.Type), &Config{
		MetricsBuilderConfig: cfg,
		HwmonPath:            base,
		Temperature:          &TemperatureConfig{},
	})
	require.NoError(t, s.start(t.Context(), nil))

	m, err := s.scrape(t.Context())
	var partialErr scrapererror.PartialScrapeError
	require.ErrorAs(t, err, &partialErr)
	assert.Equal(t, 1, partialErr.Failed)

	_, temperatures := collect(t, m, "hw.temperature")
	assert.Empty(t, temperatures)
	_, limits := collect(t, m, "hw.temperature.limit")
	require.Len(t, limits, 2)
	byType := make(map[string]float64, len(limits))
	for _, limit := range limits {
		byType[limit.attr["hw.limit_type"].(string)] = limit.val
	}
	assert.InDelta(t, 100, byType["high.critical"], 0.001)
	assert.InDelta(t, 80, byType["high.degraded"], 0.001)
}

func TestPartialError_LimitOnlyDoesNotReadTemperatureInput(t *testing.T) {
	base := t.TempDir()
	writeSensor(t, base, "hwmon0", "coretemp", "temp1", "not-a-number", "CPU", "100000", "80000")

	cfg := metadata.NewDefaultMetricsBuilderConfig()
	cfg.Metrics.HwTemperature.Enabled = false
	cfg.Metrics.HwTemperatureLimit.Enabled = true

	s := newHardwareScraper(t.Context(), scrapertest.NewNopSettings(metadata.Type), &Config{
		MetricsBuilderConfig: cfg,
		HwmonPath:            base,
		Temperature:          &TemperatureConfig{},
	})
	require.NoError(t, s.start(t.Context(), nil))

	m, err := s.scrape(t.Context())
	require.NoError(t, err)
	_, limits := collect(t, m, "hw.temperature.limit")
	assert.Len(t, limits, 2)
}
