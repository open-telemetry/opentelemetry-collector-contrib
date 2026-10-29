// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package processscraper

import (
	"runtime"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/scraper/scrapertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processscraper/internal/metadata"
)

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.IsType(t, &Config{}, cfg)
}

func TestCreateResourceMetricsScraper(t *testing.T) {
	factory := NewFactory()
	cfg := &Config{}

	scraper, err := factory.CreateMetrics(t.Context(), scrapertest.NewNopSettings(metadata.Type), cfg)

	if slices.Contains(supportedPlatforms, runtime.GOOS) {
		assert.NoError(t, err)
		assert.NotNil(t, scraper)
	} else {
		assert.ErrorIs(t, err, errUnsupportedPlatform)
		assert.Nil(t, scraper)
	}
}
