// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hostmetricsreceiver

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/otelcol/otelcoltest"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/cpuscraper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/diskscraper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/filesystemscraper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/loadscraper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/memoryscraper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/networkscraper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/pagingscraper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processesscraper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processscraper"
)

func TestLoadConfig(t *testing.T) {
	factories, err := otelcoltest.NopFactories()
	require.NoError(t, err)

	factory := NewFactory()
	factories.Receivers[metadata.Type] = factory
	cfg, err := otelcoltest.LoadConfigAndValidate(filepath.Join("testdata", "config.yaml"), factories)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, len(cfg.Receivers), 2)

	r0 := cfg.Receivers[component.NewID(metadata.Type)]
	defaultConfigCPUScraper := factory.CreateDefaultConfig()
	defaultConfigCPUScraper.(*Config).Scrapers = map[string]internal.Config{
		cpuscraper.TypeStr: (&cpuscraper.Factory{}).CreateDefaultConfig(),
	}

	assert.Equal(t, defaultConfigCPUScraper, r0)

	r1 := cfg.Receivers[component.NewIDWithName(metadata.Type, "customname")].(*Config)
	expectedConfig := &Config{
		ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
			CollectionInterval: 30 * time.Second,
			InitialDelay:       time.Second,
		},
		Scrapers: map[string]internal.Config{
			cpuscraper.TypeStr:  (&cpuscraper.Factory{}).CreateDefaultConfig(),
			diskscraper.TypeStr: (&diskscraper.Factory{}).CreateDefaultConfig(),
			loadscraper.TypeStr: (func() internal.Config {
				cfg := (&loadscraper.Factory{}).CreateDefaultConfig()
				cfg.(*loadscraper.Config).CPUAverage = true
				return cfg
			})(),
			filesystemscraper.TypeStr: (&filesystemscraper.Factory{}).CreateDefaultConfig(),
			memoryscraper.TypeStr:     (&memoryscraper.Factory{}).CreateDefaultConfig(),
			networkscraper.TypeStr: (func() internal.Config {
				cfg := (&networkscraper.Factory{}).CreateDefaultConfig()
				cfg.(*networkscraper.Config).Include = networkscraper.MatchConfig{
					Interfaces: []string{"test1"},
					Config:     filterset.Config{MatchType: "strict"},
				}
				return cfg
			})(),
			processesscraper.TypeStr: (&processesscraper.Factory{}).CreateDefaultConfig(),
			pagingscraper.TypeStr:    (&pagingscraper.Factory{}).CreateDefaultConfig(),
			processscraper.TypeStr: (func() internal.Config {
				cfg := (&processscraper.Factory{}).CreateDefaultConfig()
				cfg.(*processscraper.Config).Include = processscraper.MatchConfig{
					Names:  []string{"test2", "test3"},
					Config: filterset.Config{MatchType: "regexp"},
				}
				return cfg
			})(),
		},
	}

	assert.Equal(t, expectedConfig, r1)
}

func TestLoadInvalidConfig_NoScrapers(t *testing.T) {
	factories, err := otelcoltest.NopFactories()
	require.NoError(t, err)

	factory := NewFactory()
	factories.Receivers[metadata.Type] = factory
	_, err = otelcoltest.LoadConfigAndValidate(filepath.Join("testdata", "config-noscrapers.yaml"), factories)

	require.Contains(t, err.Error(), "must specify at least one scraper when using hostmetrics receiver")
}

func TestLoadInvalidConfig_InvalidScraperKey(t *testing.T) {
	factories, err := otelcoltest.NopFactories()
	require.NoError(t, err)

	factory := NewFactory()
	factories.Receivers[metadata.Type] = factory
	_, err = otelcoltest.LoadConfigAndValidate(filepath.Join("testdata", "config-invalidscraperkey.yaml"), factories)

	require.Contains(t, err.Error(), "error reading configuration for \"hostmetrics\": invalid scraper key: invalidscraperkey")
}
