// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package raidscraper

import (
	"context"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/scraper/scrapertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/raidscraper/internal/metadata"
)

func TestCreateRaidScraper(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)

	scraper, err := factory.CreateMetrics(context.Background(), scrapertest.NewNopSettings(metadata.Type), cfg)

	if runtime.GOOS == "linux" || runtime.GOOS == "darwin" {
		assert.Equal(t, defaultSysDeviceFilesystem, cfg.SysDeviceFilesystem)
		assert.NoError(t, err)
		assert.NotNil(t, scraper)
	} else {
		assert.Error(t, err)
		assert.Nil(t, scraper)
	}

}
