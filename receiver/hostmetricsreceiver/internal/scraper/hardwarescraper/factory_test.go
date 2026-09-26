// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hardwarescraper

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/scraper/scrapertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/hardwarescraper/internal/metadata"
)

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	require.IsType(t, &Config{}, cfg)
	defaultConfig := cfg.(*Config)
	require.NotNil(t, defaultConfig.Temperature)
	assert.Empty(t, defaultConfig.Temperature.Include.Sensors)
	assert.Equal(t, filterset.Regexp, defaultConfig.Temperature.Include.MatchType)
	assert.Empty(t, defaultConfig.Temperature.Exclude.Sensors)
}

func TestCreateMetrics(t *testing.T) {
	factory := NewFactory()
	cfg := &Config{}

	scraper, err := factory.CreateMetrics(t.Context(), scrapertest.NewNopSettings(metadata.Type), cfg)

	if supportedOS {
		assert.NoError(t, err)
		assert.NotNil(t, scraper)
	} else {
		assert.ErrorIs(t, err, errUnsupportedOS)
		assert.Nil(t, scraper)
	}
}
