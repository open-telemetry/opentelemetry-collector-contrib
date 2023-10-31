// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oracledbreceiver // import "github.com/open-telemetry/open-telemetry-collector-contrib/receiver/oracledbreceiver"

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/oracledbreceiver/internal/metadata"
)

func TestValidateInvalidConfigs(t *testing.T) {
	testCases := []struct {
		name     string
		config   *Config
		expected error
	}{
		{
			name: "Empty endpoint",
			config: &Config{
				Endpoint:                  "",
				ScraperControllerSettings: scraperhelper.NewDefaultScraperControllerSettings(metadata.Type),
			},
			expected: errEmptyEndpoint,
		},
		{
			name: "Missing port in endpoint",
			config: &Config{
				Endpoint:                  "localhost",
				ScraperControllerSettings: scraperhelper.NewDefaultScraperControllerSettings(metadata.Type),
			},
			expected: errBadEndpoint,
		},
		{
			name: "Invalid endpoint format",
			config: &Config{
				Endpoint:                  "x;;ef;s;d:::ss:23423423423423423",
				ScraperControllerSettings: scraperhelper.NewDefaultScraperControllerSettings(metadata.Type),
			},
			expected: errBadEndpoint,
		},
		{
			name: "Missing host in endpoint",
			config: &Config{
				Endpoint:                  ":3001",
				ScraperControllerSettings: scraperhelper.NewDefaultScraperControllerSettings(metadata.Type),
			},
			expected: errBadEndpoint,
		},
		{
			name: "Negative port",
			config: &Config{
				Endpoint:                  "localhost:-2",
				ScraperControllerSettings: scraperhelper.NewDefaultScraperControllerSettings(metadata.Type),
			},
			expected: errBadPort,
		},
		{
			name: "Bad port",
			config: &Config{
				Endpoint:                  "localhost:9999999999999999999",
				ScraperControllerSettings: scraperhelper.NewDefaultScraperControllerSettings(metadata.Type),
			},
			expected: errBadPort,
		},
		{
			name: "Empty username",
			config: &Config{
				Endpoint:                  "localhost:3000",
				Username:                  "",
				Password:                  "secret",
				ScraperControllerSettings: scraperhelper.NewDefaultScraperControllerSettings(metadata.Type),
			},
			expected: errEmptyUsername,
		},
		{
			name: "Empty password",
			config: &Config{
				Endpoint:                  "localhost:3000",
				Username:                  "ro_user",
				ScraperControllerSettings: scraperhelper.NewDefaultScraperControllerSettings(metadata.Type),
			},
			expected: errEmptyPassword,
		},
		{
			name: "Empty service",
			config: &Config{
				Endpoint:                  "localhost:3000",
				Password:                  "password",
				Username:                  "ro_user",
				ScraperControllerSettings: scraperhelper.NewDefaultScraperControllerSettings(metadata.Type),
			},
			expected: errEmptyService,
		},
		{
			name: "Invalid data source",
			config: &Config{
				DataSource:                "%%%",
				ScraperControllerSettings: scraperhelper.NewDefaultScraperControllerSettings(metadata.Type),
			},
			expected: errBadDataSource,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.config.Validate()
			require.ErrorIs(t, err, tc.expected)
		})
	}
}

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	assert.Equal(t, 10*time.Second, cfg.ScraperControllerSettings.CollectionInterval)
}

func TestParseConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	sub, err := cm.Sub("oracledb")
	require.NoError(t, err)
	cfg := createDefaultConfig().(*Config)

	require.NoError(t, err)
	require.NoError(t, component.UnmarshalConfig(sub, cfg))
	assert.Equal(t, "oracle://otel:password@localhost:51521/XE", cfg.DataSource)
	assert.Equal(t, "otel", cfg.Username)
	assert.Equal(t, "password", cfg.Password)
	assert.Equal(t, "localhost:51521", cfg.Endpoint)
	assert.Equal(t, "XE", cfg.Service)
	settings := cfg.MetricsBuilderConfig.Metrics
	assert.False(t, settings.OracledbTablespaceSizeUsage.Enabled)
	assert.False(t, settings.OracledbExchangeDeadlocks.Enabled)
}
