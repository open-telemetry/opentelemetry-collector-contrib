// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sqlserverreceiver

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlserverreceiver/internal/metadata"
)

func ptr[T any](v T) *T { return new(v) }

func TestValidate(t *testing.T) {
	testCases := []struct {
		desc            string
		cfg             *Config
		expectedSuccess bool
	}{
		{
			desc: "valid config",
			cfg: &Config{
				MetricsBuilderConfig: metadata.NewDefaultMetricsBuilderConfig(),
				ControllerConfig:     scraperhelper.NewDefaultControllerConfig(),
			},
			expectedSuccess: true,
		},
		{
			desc: "valid config with no metric settings",
			cfg: &Config{
				MetricsBuilderConfig: metadata.NewDefaultMetricsBuilderConfig(),
				ControllerConfig:     scraperhelper.NewDefaultControllerConfig(),
			},
			expectedSuccess: true,
		},
		{
			desc:            "default config is valid",
			cfg:             createDefaultConfig().(*Config),
			expectedSuccess: true,
		},
		{
			desc: "invalid config with partial direct connect settings",
			cfg: &Config{
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
				Server:           "0.0.0.0",
				Username:         "sa",
			},
			expectedSuccess: false,
		},
		{
			desc: "invalid config with datasource and any direct connect settings",
			cfg: &Config{
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
				DataSource:       "a connection string",
				Username:         "sa",
				Port:             1433,
			},
			expectedSuccess: false,
		},
		{
			desc: "valid config only datasource and none direct connect settings",
			cfg: &Config{
				MetricsBuilderConfig: metadata.NewDefaultMetricsBuilderConfig(),
				ControllerConfig:     scraperhelper.NewDefaultControllerConfig(),
				DataSource:           "a connection string",
			},
			expectedSuccess: true,
		},
		{
			desc: "valid config with all direct connection settings",
			cfg: &Config{
				MetricsBuilderConfig: metadata.NewDefaultMetricsBuilderConfig(),
				ControllerConfig:     scraperhelper.NewDefaultControllerConfig(),
				Server:               "0.0.0.0",
				Username:             "sa",
				Password:             "password",
				Port:                 1433,
			},
			expectedSuccess: true,
		},
		{
			desc: "config with invalid MaxQuerySampleCount value",
			cfg: &Config{
				MetricsBuilderConfig: metadata.NewDefaultMetricsBuilderConfig(),
				ControllerConfig:     scraperhelper.NewDefaultControllerConfig(),
				TopQueryCollection: TopQueryCollection{
					MaxQuerySampleCount: 100000,
				},
			},
			expectedSuccess: false,
		},
		{
			desc: "config with invalid TopQueryCount value",
			cfg: &Config{
				MetricsBuilderConfig: metadata.NewDefaultMetricsBuilderConfig(),
				ControllerConfig:     scraperhelper.NewDefaultControllerConfig(),
				TopQueryCollection: TopQueryCollection{
					MaxQuerySampleCount: 100,
					TopQueryCount:       200000,
				},
			},
			expectedSuccess: false,
		},
		{
			desc: "config with invalid LookbackTime",
			cfg: &Config{
				MetricsBuilderConfig: metadata.NewDefaultMetricsBuilderConfig(),
				ControllerConfig:     scraperhelper.NewDefaultControllerConfig(),
				TopQueryCollection: TopQueryCollection{
					MaxQuerySampleCount: 100,
					TopQueryCount:       200000,
					LookbackTime:        -1,
				},
			},
			expectedSuccess: false,
		},
		{
			desc: "config with negative connection_pool.max_open",
			cfg: &Config{
				MetricsBuilderConfig: metadata.NewDefaultMetricsBuilderConfig(),
				ControllerConfig:     scraperhelper.NewDefaultControllerConfig(),
				ConnectionPool:       ConnectionPool{MaxOpen: new(-1)},
			},
			expectedSuccess: false,
		},
		{
			desc: "config with negative connection_pool.max_idle_time",
			cfg: &Config{
				MetricsBuilderConfig: metadata.NewDefaultMetricsBuilderConfig(),
				ControllerConfig:     scraperhelper.NewDefaultControllerConfig(),
				ConnectionPool:       ConnectionPool{MaxIdleTime: ptr(-1 * time.Second)},
			},
			expectedSuccess: false,
		},
		{
			desc: "config with valid connection_pool",
			cfg: &Config{
				MetricsBuilderConfig: metadata.NewDefaultMetricsBuilderConfig(),
				ControllerConfig:     scraperhelper.NewDefaultControllerConfig(),
				ConnectionPool: ConnectionPool{
					MaxOpen:     new(8),
					MaxIdle:     new(4),
					MaxLifetime: ptr(5 * time.Minute),
					MaxIdleTime: ptr(time.Minute),
				},
			},
			expectedSuccess: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			if tc.expectedSuccess {
				require.NoError(t, confmap.Validate(tc.cfg))
			} else {
				require.Error(t, confmap.Validate(tc.cfg))
			}
		})
	}
}

func TestLoadConfig(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
		require.NoError(t, err)
		factory := NewFactory()
		cfg := factory.CreateDefaultConfig()

		sub, err := cm.Sub("sqlserver")
		require.NoError(t, err)
		require.NoError(t, sub.Unmarshal(cfg))

		assert.NoError(t, confmap.Validate(cfg))
		assert.Equal(t, factory.CreateDefaultConfig(), cfg)
	})

	t.Run("named", func(t *testing.T) {
		cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
		require.NoError(t, err)

		factory := NewFactory()
		cfg := factory.CreateDefaultConfig()

		expected := factory.CreateDefaultConfig().(*Config)
		expected.MetricsBuilderConfig = metadata.MetricsBuilderConfig{
			Metrics: metadata.DefaultMetricsConfig(),
			ResourceAttributes: metadata.ResourceAttributesConfig{
				HostName: metadata.HostNameResourceAttributeConfig{
					Enabled: true,
				},
				ServiceName: metadata.ServiceNameResourceAttributeConfig{
					Enabled: true,
				},
				SqlserverDatabaseName: metadata.SqlserverDatabaseNameResourceAttributeConfig{
					Enabled: true,
				},
				SqlserverInstanceName: metadata.SqlserverInstanceNameResourceAttributeConfig{
					Enabled: true,
				},
				SqlserverComputerName: metadata.SqlserverComputerNameResourceAttributeConfig{
					Enabled: true,
				},
				ServerAddress: metadata.ServerAddressResourceAttributeConfig{
					Enabled: true,
				},
				ServerPort: metadata.ServerPortResourceAttributeConfig{
					Enabled: true,
				},
			},
		}
		expected.LogsBuilderConfig = metadata.LogsBuilderConfig{
			Events: metadata.EventsConfig{
				DbServerQuerySample: metadata.EventConfig{
					Enabled: true,
				},
				DbServerTopQuery: metadata.EventConfig{
					Enabled: true,
				},
			},
			ResourceAttributes: metadata.ResourceAttributesConfig{
				HostName: metadata.HostNameResourceAttributeConfig{
					Enabled: true,
				},
				ServiceName: metadata.ServiceNameResourceAttributeConfig{
					Enabled: true,
				},
				SqlserverDatabaseName: metadata.SqlserverDatabaseNameResourceAttributeConfig{
					Enabled: true,
				},
				SqlserverInstanceName: metadata.SqlserverInstanceNameResourceAttributeConfig{
					Enabled: true,
				},
				SqlserverComputerName: metadata.SqlserverComputerNameResourceAttributeConfig{
					Enabled: true,
				},
				ServerAddress: metadata.ServerAddressResourceAttributeConfig{
					Enabled: true,
				},
				ServerPort: metadata.ServerPortResourceAttributeConfig{
					Enabled: true,
				},
			},
		}
		expected.ComputerName = "CustomServer"
		expected.InstanceName = "CustomInstance"
		expected.TopQueryCollection.LookbackTime = 60 * time.Second
		expected.TopQueryCollection.TopQueryCount = 200
		expected.TopQueryCollection.MaxQuerySampleCount = 1000
		expected.TopQueryCollection.CollectionInterval = 80 * time.Second

		expected.QuerySample = QuerySample{
			MaxRowsPerQuery: 1450,
		}

		sub, err := cm.Sub("sqlserver/named")
		require.NoError(t, err)
		require.NoError(t, sub.Unmarshal(cfg))

		assert.NoError(t, confmap.Validate(cfg))
		if diff := cmp.Diff(expected, cfg, cmp.FilterPath(func(p cmp.Path) bool {
			if sf, ok := p.Last().(cmp.StructField); ok {
				name := sf.Name()
				return name != "" && name[0] >= 'a' && name[0] <= 'z'
			}
			return false
		}, cmp.Ignore())); diff != "" {
			t.Errorf("Config mismatch (-expected +actual):\n%s", diff)
		}
	})

	t.Run("effectiveLookBackTime", func(t *testing.T) {
		factory := NewFactory()
		config := factory.CreateDefaultConfig().(*Config)

		config.TopQueryCollection.CollectionInterval = 10 * time.Second
		assert.Equal(t, 2*config.TopQueryCollection.CollectionInterval, config.EffectiveLookbackTime(), "By default the 'EffectiveLookbackTime' value should be 2 x 'TopQueryCollection.CollectionInterval'")

		config.TopQueryCollection.LookbackTime = 60 * time.Second
		assert.Equal(t, 60*time.Second, config.EffectiveLookbackTime(), "'EffectiveLookbackTime' should return the user provided 'LookbackTime' if any.")
	})
}
