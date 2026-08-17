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
	"go.opentelemetry.io/collector/scraper/scrapertest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/hardwarescraper/internal/metadata"
)

// scrapeCount runs one scrape against a started scraper and returns how many
// hw.temperature data points it produced.
func scrapeCount(t *testing.T, s *hardwareTemperatureScraper, cfg metadata.MetricsBuilderConfig) int {
	t.Helper()
	mb := metadata.NewMetricsBuilder(cfg, scrapertest.NewNopSettings(metadata.Type))
	require.NoError(t, s.scrape(t.Context(), mb))
	_, pts := collect(t, mb.Emit(), "hw.temperature")
	return len(pts)
}

// Sensors must be enumerated per scrape, not cached at start, so that hwmon
// devices appearing or disappearing at runtime are reflected.
func TestRescan_SensorAppearsAndDisappearsBetweenScrapes(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Test is for Linux platform")
	}

	base := t.TempDir()
	writeSensor(t, base, "hwmon0", "coretemp", "temp1", "47000", "Package id 0", "", "")

	cfg := metadata.NewDefaultMetricsBuilderConfig()
	cfg.Metrics.HwTemperature.Enabled = true

	s := &hardwareTemperatureScraper{
		logger:               zap.NewNop(),
		config:               &TemperatureConfig{},
		hwmonPath:            base,
		metricsBuilderConfig: cfg,
	}
	require.NoError(t, s.start(t.Context()))

	assert.Equal(t, 1, scrapeCount(t, s, cfg), "the sensor present at start is reported")

	writeSensor(t, base, "hwmon1", "nvme", "temp1", "40850", "Composite", "", "")
	assert.Equal(t, 2, scrapeCount(t, s, cfg), "a sensor appearing after start is picked up")

	require.NoError(t, os.RemoveAll(filepath.Join(base, "hwmon0")))
	assert.Equal(t, 1, scrapeCount(t, s, cfg), "a sensor that disappeared is dropped without an error")
}

// A hwmon path that does not exist at start must not prevent sensors from being
// reported once it appears.
func TestRescan_HwmonPathAppearsAfterStart(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Test is for Linux platform")
	}

	root := t.TempDir()
	hwmonPath := filepath.Join(root, "hwmon-not-yet-there")

	cfg := metadata.NewDefaultMetricsBuilderConfig()
	cfg.Metrics.HwTemperature.Enabled = true

	s := &hardwareTemperatureScraper{
		logger:               zap.NewNop(),
		config:               &TemperatureConfig{},
		hwmonPath:            hwmonPath,
		metricsBuilderConfig: cfg,
	}
	require.NoError(t, s.start(t.Context()))

	assert.Equal(t, 0, scrapeCount(t, s, cfg), "no hwmon path means no metrics and no error")

	writeSensor(t, hwmonPath, "hwmon0", "acpitz", "temp1", "51000", "", "", "")
	assert.Equal(t, 1, scrapeCount(t, s, cfg), "the sensor is reported once the path appears")
}
