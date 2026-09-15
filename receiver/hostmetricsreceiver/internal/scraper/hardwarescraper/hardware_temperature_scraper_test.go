// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !linux

package hardwarescraper

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/scraper/scrapertest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/hardwarescraper/internal/metadata"
)

func TestHardwareTemperatureScraperStart(t *testing.T) {
	scraper := &hardwareTemperatureScraper{
		logger:               zap.NewNop(),
		config:               &TemperatureConfig{},
		hwmonPath:            "/sys/class/hwmon",
		metricsBuilderConfig: metadata.NewDefaultMetricsBuilderConfig(),
	}

	err := scraper.start(t.Context())
	assert.Error(t, err)
	assert.Equal(t, ErrHwmonUnavailable, err)
}

func TestHardwareTemperatureScraperScrape(t *testing.T) {
	scraper := &hardwareTemperatureScraper{
		logger:               zap.NewNop(),
		config:               &TemperatureConfig{},
		hwmonPath:            "/sys/class/hwmon",
		metricsBuilderConfig: metadata.NewDefaultMetricsBuilderConfig(),
	}

	mb := metadata.NewMetricsBuilder(metadata.NewDefaultMetricsBuilderConfig(), scrapertest.NewNopSettings(metadata.Type))
	err := scraper.scrape(t.Context(), mb)
	assert.Error(t, err)
	assert.Equal(t, ErrHwmonUnavailable, err)
}
