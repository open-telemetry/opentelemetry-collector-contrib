// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package groupprocessscraper

import (
	"context"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

func TestCreateDefaultConfig(t *testing.T) {
	factory := &Factory{}
	cfg := factory.CreateDefaultConfig()
	assert.IsType(t, &Config{}, cfg)
}

func TestCreateResourceMetricsScraper(t *testing.T) {
	factory := &Factory{}
	cfg := &Config{}

	scraper, err := factory.CreateMetricsScraper(context.Background(), receivertest.NewNopSettings(), cfg)

	if runtime.GOOS == "linux" || runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		assert.NoError(t, err)
		assert.NotNil(t, scraper)
		assert.Equal(t, scraperType.String(), scraper.ID().String())
	} else {
		assert.Error(t, err)
		assert.Nil(t, scraper)
	}
}
