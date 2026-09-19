// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package hardwarescraper

import (
	"runtime"
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
	if runtime.GOOS != "linux" {
		t.Skip("Test is for Linux platform")
	}

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
	assert.Equal(t, 2*hardwareTemperatureMetricsLen, partialErr.Failed,
		"two failed sensors must be counted twice, not collapsed into one wrapper count")

	_, pts := collect(t, m, "hw.temperature")
	assert.Len(t, pts, 1, "the healthy sensor is still reported")
}
