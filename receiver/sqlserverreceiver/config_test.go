// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sqlserverreceiver

import (
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlserverreceiver/internal/metadata"
)

func TestValidate(t *testing.T) {
	testCases := []struct {
		desc            string
		cfg             *Config
		expectedSuccess bool
	}{
		{
			desc: "valid config",
			cfg: &Config{
				MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
				ControllerConfig:     scraperhelper.NewDefaultControllerConfig(),
			},
			expectedSuccess: true,
		},
		{
			desc: "valid config with no metric settings",
			cfg: &Config{
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
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
			desc: "valid config with all direct connection settings",
			cfg: &Config{
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
				Server:           "0.0.0.0",
				Username:         "sa",
				Password:         "password",
				Port:             1433,
			},
			expectedSuccess: true,
		},
		{
			desc: "config with invalid MaxQuerySampleCount value",
			cfg: &Config{
				MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
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
				MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
				ControllerConfig:     scraperhelper.NewDefaultControllerConfig(),
				TopQueryCollection: TopQueryCollection{
					MaxQuerySampleCount: 100,
					TopQueryCount:       200000,
				},
			},
			expectedSuccess: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			if tc.expectedSuccess {
				require.NoError(t, xconfmap.Validate(tc.cfg))
			} else {
				require.Error(t, xconfmap.Validate(tc.cfg))
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

		assert.NoError(t, xconfmap.Validate(cfg))
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
				SqlserverDatabaseName: metadata.ResourceAttributeConfig{
					Enabled: true,
				},
				SqlserverInstanceName: metadata.ResourceAttributeConfig{
					Enabled: true,
				},
				SqlserverComputerName: metadata.ResourceAttributeConfig{
					Enabled: true,
				},
				ServerAddress: metadata.ResourceAttributeConfig{
					Enabled: true,
				},
				ServerPort: metadata.ResourceAttributeConfig{
					Enabled: true,
				},
			},
		}
		expected.ComputerName = "CustomServer"
		expected.InstanceName = "CustomInstance"
		expected.Enabled = true
		expected.LookbackTime = 60
		expected.TopQueryCount = 200
		expected.TopQueryCollection.MaxQuerySampleCount = 1000

		sub, err := cm.Sub("sqlserver/named")
		require.NoError(t, err)
		require.NoError(t, sub.Unmarshal(cfg))

		assert.NoError(t, xconfmap.Validate(cfg))
		if diff := cmp.Diff(expected, cfg, cmpopts.IgnoreUnexported(metadata.MetricConfig{}), cmpopts.IgnoreUnexported(metadata.ResourceAttributeConfig{})); diff != "" {
			t.Errorf("Config mismatch (-expected +actual):\n%s", diff)
		}
	})
}
