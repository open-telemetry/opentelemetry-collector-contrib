// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !linux

package hwscraper

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/scraper/scrapertest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/hwscraper/internal/metadata"
)

func TestHwTemperatureScraperStart(t *testing.T) {
	scraper := &hwTemperatureScraper{
		logger:               zap.NewNop(),
		config:               &TemperatureConfig{},
		hwmonPath:            "/sys/class/hwmon",
		metricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
	}

	err := scraper.start(t.Context())
	assert.Error(t, err)
	assert.Equal(t, ErrHWMonUnavailable, err)
}

func TestHwTemperatureScraperScrape(t *testing.T) {
	scraper := &hwTemperatureScraper{
		logger:               zap.NewNop(),
		config:               &TemperatureConfig{},
		hwmonPath:            "/sys/class/hwmon",
		metricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
	}

	mb := metadata.NewMetricsBuilder(metadata.DefaultMetricsBuilderConfig(), scrapertest.NewNopSettings(metadata.Type))
	err := scraper.scrape(t.Context(), mb)
	assert.Error(t, err)
	assert.Equal(t, ErrHWMonUnavailable, err)
}
