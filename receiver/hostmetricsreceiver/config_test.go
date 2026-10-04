// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hostmetricsreceiver

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/cpuscraper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/diskscraper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/filesystemscraper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/hardwarescraper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/loadscraper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/memoryscraper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/networkscraper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/nfsscraper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/pagingscraper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processesscraper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/processscraper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver/internal/scraper/systemscraper"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id       component.ID
		expected component.Config
	}{
		{
			id: component.NewID(metadata.Type),
			expected: func() component.Config {
				cfg := createDefaultConfig().(*Config)
				cpu := cpuscraper.NewFactory()
				cfg.Scrapers = map[component.Type]component.Config{
					cpu.Type(): func() component.Config {
						return cpu.CreateDefaultConfig()
					}(),
				}
				return cfg
			}(),
		},
		{
			id: component.NewIDWithName(metadata.Type, "customname"),
			expected: &Config{
				MetadataCollectionInterval: 5 * time.Minute,
				ControllerConfig: scraperhelper.ControllerConfig{
					CollectionInterval: 30 * time.Second,
					InitialDelay:       time.Second,
				},
				Scrapers: map[component.Type]component.Config{
					component.MustNewType("cpu"):  cpuscraper.NewFactory().CreateDefaultConfig(),
					component.MustNewType("disk"): diskscraper.NewFactory().CreateDefaultConfig(),
					component.MustNewType("load"): func() component.Config {
						cfg := loadscraper.NewFactory().CreateDefaultConfig()
						cfg.(*loadscraper.Config).CPUAverage = true
						return cfg
					}(),
					component.MustNewType("filesystem"): filesystemscraper.NewFactory().CreateDefaultConfig(),
					component.MustNewType("memory"):     memoryscraper.NewFactory().CreateDefaultConfig(),
					component.MustNewType("network"): func() component.Config {
						cfg := networkscraper.NewFactory().CreateDefaultConfig()
						cfg.(*networkscraper.Config).Include = networkscraper.MatchConfig{
							Interfaces: []string{"test1"},
							Config:     filterset.Config{MatchType: "strict"},
						}
						return cfg
					}(),
					component.MustNewType("nfs"):       nfsscraper.NewFactory().CreateDefaultConfig(),
					component.MustNewType("processes"): processesscraper.NewFactory().CreateDefaultConfig(),
					component.MustNewType("paging"):    pagingscraper.NewFactory().CreateDefaultConfig(),
					component.MustNewType("process"): func() component.Config {
						cfg := processscraper.NewFactory().CreateDefaultConfig()
						cfg.(*processscraper.Config).Include = processscraper.MatchConfig{
							Names:  []string{"test2", "test3"},
							Config: filterset.Config{MatchType: "regexp"},
						}
						return cfg
					}(),
					component.MustNewType("system"): systemscraper.NewFactory().CreateDefaultConfig(),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			require.NoError(t, confmap.Validate(cfg))
			require.Equal(t, tt.expected, cfg)
		})
	}
}

func TestLoadDeprecatedConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config-deprecated.yaml"))
	require.NoError(t, err)

	deprecatedType := component.MustNewType("hostmetrics")

	tests := []struct {
		id       component.ID
		expected component.Config
	}{
		{
			id: component.NewID(deprecatedType),
			expected: func() component.Config {
				cfg := createDefaultConfig().(*Config)
				cpu := cpuscraper.NewFactory()
				cfg.Scrapers = map[component.Type]component.Config{
					cpu.Type(): func() component.Config {
						return cpu.CreateDefaultConfig()
					}(),
				}
				return cfg
			}(),
		},
		{
			id: component.NewIDWithName(deprecatedType, "customname"),
			expected: &Config{
				MetadataCollectionInterval: 5 * time.Minute,
				ControllerConfig: scraperhelper.ControllerConfig{
					CollectionInterval: 30 * time.Second,
					InitialDelay:       time.Second,
				},
				Scrapers: map[component.Type]component.Config{
					component.MustNewType("cpu"):  cpuscraper.NewFactory().CreateDefaultConfig(),
					component.MustNewType("disk"): diskscraper.NewFactory().CreateDefaultConfig(),
					component.MustNewType("load"): func() component.Config {
						cfg := loadscraper.NewFactory().CreateDefaultConfig()
						cfg.(*loadscraper.Config).CPUAverage = true
						return cfg
					}(),
					component.MustNewType("filesystem"): filesystemscraper.NewFactory().CreateDefaultConfig(),
					component.MustNewType("memory"):     memoryscraper.NewFactory().CreateDefaultConfig(),
					component.MustNewType("network"): func() component.Config {
						cfg := networkscraper.NewFactory().CreateDefaultConfig()
						cfg.(*networkscraper.Config).Include = networkscraper.MatchConfig{
							Interfaces: []string{"test1"},
							Config:     filterset.Config{MatchType: "strict"},
						}
						return cfg
					}(),
					component.MustNewType("nfs"):       nfsscraper.NewFactory().CreateDefaultConfig(),
					component.MustNewType("processes"): processesscraper.NewFactory().CreateDefaultConfig(),
					component.MustNewType("paging"):    pagingscraper.NewFactory().CreateDefaultConfig(),
					component.MustNewType("process"): func() component.Config {
						cfg := processscraper.NewFactory().CreateDefaultConfig()
						cfg.(*processscraper.Config).Include = processscraper.MatchConfig{
							Names:  []string{"test2", "test3"},
							Config: filterset.Config{MatchType: "regexp"},
						}
						return cfg
					}(),
					component.MustNewType("system"): systemscraper.NewFactory().CreateDefaultConfig(),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			require.NoError(t, confmap.Validate(cfg))
			require.Equal(t, tt.expected, cfg)
		})
	}
}

func TestLoadInvalidConfig_NoScrapers(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config-noscrapers.yaml"))
	require.NoError(t, err)

	require.NoError(t, cm.Unmarshal(cfg))
	require.ErrorContains(t, confmap.Validate(cfg), "must specify at least one scraper when using host_metrics receiver")
}

func TestConfigValidate_MetadataCollectionInterval(t *testing.T) {
	tests := []struct {
		name        string
		interval    time.Duration
		expectError bool
	}{
		{
			name:        "invalid interval - negative",
			interval:    -time.Second,
			expectError: true,
		},
		{
			name:        "valid interval - zero",
			interval:    0,
			expectError: false,
		},
		{
			name:        "valid interval - positive",
			interval:    time.Second,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cpu := cpuscraper.NewFactory()
			cfg := createDefaultConfig().(*Config)
			cfg.MetadataCollectionInterval = tt.interval
			cfg.Scrapers = map[component.Type]component.Config{
				cpu.Type(): cpu.CreateDefaultConfig(),
			}

			err := confmap.Validate(cfg)
			if tt.expectError {
				require.ErrorContains(t, err, "metadata_collection_interval must not be negative")
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestLoadInvalidConfig_InvalidScraperKey(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config-invalidscraperkey.yaml"))
	require.NoError(t, err)

	require.ErrorContains(t, cm.Unmarshal(cfg), "invalid scraper key: invalidscraperkey")
}

func TestLoadHardwareConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config-hardware.yaml"))
	require.NoError(t, err)

	t.Run("receiver root with nested temperature filters", func(t *testing.T) {
		sub, subErr := cm.Sub("hostmetrics")
		require.NoError(t, subErr)
		cfg := createDefaultConfig().(*Config)
		require.NoError(t, sub.Unmarshal(cfg))
		require.Equal(t, "/hostfs", cfg.RootPath)
		hardware, ok := cfg.Scrapers[component.MustNewType("hardware")].(*hardwarescraper.Config)
		require.True(t, ok)
		expected := hardwarescraper.NewFactory().CreateDefaultConfig().(*hardwarescraper.Config)
		expected.Temperature.Include.Sensors = []string{"Core.*"}
		require.Equal(t, expected, hardware)
	})

	t.Run("removed hwmon_path is rejected", func(t *testing.T) {
		sub, subErr := cm.Sub("hostmetrics/removed-path")
		require.NoError(t, subErr)
		require.ErrorContains(t, sub.Unmarshal(createDefaultConfig()), "invalid keys: hwmon_path")
	})
}
