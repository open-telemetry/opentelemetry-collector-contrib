// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filesystemscraper

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/scraper/scrapertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/filesystemscraper/internal/metadata"
)

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.IsType(t, &Config{}, cfg)
}

func TestCreateMetrics(t *testing.T) {
	factory := NewFactory()
	cfg := &Config{}

	scraper, err := factory.CreateMetrics(t.Context(), scrapertest.NewNopSettings(metadata.Type), cfg)

	assert.NoError(t, err)
	assert.NotNil(t, scraper)
}

func TestCreateMetrics_Error(t *testing.T) {
	factory := NewFactory()
	cfg := &Config{IncludeDevices: DeviceMatchConfig{Devices: []string{""}}}

	_, err := factory.CreateMetrics(t.Context(), scrapertest.NewNopSettings(metadata.Type), cfg)

	assert.Error(t, err)
}
