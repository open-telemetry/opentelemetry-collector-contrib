// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pagingscraper

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/scraper/scrapertest"
)

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.IsType(t, &Config{}, cfg)
}

func TestCreateMetrics(t *testing.T) {
	factory := NewFactory()
	cfg := &Config{}

	scraper, err := factory.CreateMetrics(context.Background(), scrapertest.NewNopSettings(), cfg)
	assert.NoError(t, err)
	assert.NotNil(t, scraper)
}
