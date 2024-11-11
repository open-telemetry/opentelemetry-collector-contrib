// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package memoryscraper

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

func TestCreateDefaultConfig(t *testing.T) {
	factory := &Factory{}
	cfg := factory.CreateDefaultConfig()
	assert.IsType(t, &Config{}, cfg)
}

func TestCreateMetricsScraper(t *testing.T) {
	factory := &Factory{}
	cfg := &Config{}

	scraper, err := factory.CreateMetricsScraper(context.Background(), receivertest.NewNopSettings(), cfg)

	assert.NoError(t, err)
	assert.NotNil(t, scraper)
	assert.Equal(t, scraperType.String(), scraper.ID().String())
}
