// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package redisreceiver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/scraper"
	"go.opentelemetry.io/collector/scraper/xscraper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/redisreceiver/internal/metadata"
)

func TestNewScraperFactory(t *testing.T) {
	f := NewScraperFactory()
	assert.Equal(t, metadata.Type, f.Type())
	require.NoError(t, componenttest.CheckConfigStruct(f.CreateDefaultConfig()))

	xf, ok := f.(xscraper.Factory)
	require.True(t, ok)
	assert.Equal(t, metadata.DiscoveryRule(), xf.DiscoveryRule())
	assert.False(t, xf.DiscoveryRule().IsEmpty())

	cfg := f.CreateDefaultConfig().(*ScraperConfig)
	cfg.AddrConfig.Endpoint = "localhost:6379"
	s, err := f.CreateMetrics(t.Context(), scraper.Settings{ID: component.NewID(f.Type()), TelemetrySettings: componenttest.NewNopTelemetrySettings()}, cfg)
	require.NoError(t, err)
	assert.NotNil(t, s)
}

func TestReceiverDefaultConfigMatchesScraper(t *testing.T) {
	assert.Equal(t, NewScraperFactory().CreateDefaultConfig(), &createDefaultConfig().(*Config).ScraperConfig)
}
