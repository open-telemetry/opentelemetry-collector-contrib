// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package diskscraper

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/scraper/scrapertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/diskscraper/internal/metadata"
)

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.IsType(t, &Config{}, cfg)
}

func TestCreateMetrics(t *testing.T) {
	factory := NewFactory()
	cfg := &Config{}

	scraper, err := factory.CreateMetrics(context.Background(), scrapertest.NewNopSettings(metadata.Type), cfg)

	assert.NoError(t, err)
	assert.NotNil(t, scraper)
}

func TestCreateMetrics_Error(t *testing.T) {
	factory := NewFactory()
	cfg := &Config{Include: MatchConfig{Devices: []string{""}}}

	_, err := factory.CreateMetrics(context.Background(), scrapertest.NewNopSettings(metadata.Type), cfg)

	assert.Error(t, err)
}
