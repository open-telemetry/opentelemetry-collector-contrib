//go:build !linux

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package edacscraper

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/scraper/scrapertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/edacscraper/internal/metadata"
)

func TestScrape_NonLinux(t *testing.T) {
	config := &Config{MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig()}
	scraper := newEDACMetricsScraper(context.Background(), scrapertest.NewNopSettings(metadata.Type), config)

	// Start should return error on non-Linux platforms
	err := scraper.start(context.Background(), componenttest.NewNopHost())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "EDAC scraper is only supported on Linux")

	// Scrape should also return error on non-Linux platforms
	_, err = scraper.scrape(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "EDAC scraper is only supported on Linux")
}

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NotNil(t, cfg)
	// Note: Config doesn't have Validate method, but factory creation validates
}

func TestCreateMetrics(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	scraper, err := factory.CreateMetrics(context.Background(), scrapertest.NewNopSettings(metadata.Type), cfg)
	assert.NoError(t, err)
	assert.NotNil(t, scraper)
}
