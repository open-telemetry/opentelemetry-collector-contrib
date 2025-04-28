// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package processscraper

import (
	"context"
	"runtime"
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

	scraper, err := factory.CreateMetrics(context.Background(), scrapertest.NewNopSettings(metadata.Type), cfg)

	if runtime.GOOS == "linux" || runtime.GOOS == "windows" || runtime.GOOS == "darwin" || runtime.GOOS == "freebsd" {
		assert.NoError(t, err)
		assert.NotNil(t, scraper)
	} else {
		assert.Error(t, err)
		assert.Nil(t, scraper)
	}
}
