// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package vcenterreceiver // import github.com/open-telemetry/opentelemetry-collector-contrib/receiver/vcenterreceiver

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/vcenterreceiver/internal/metadata"
)

func TestConfigValidation(t *testing.T) {
	cases := []struct {
		desc        string
		cfg         Config
		expectedErr error
	}{
		{
			desc: "empty endpoint",
			cfg: Config{
				Endpoint:         "",
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedErr: errors.New("no endpoint was provided"),
		},
		{
			desc: "with endpoint",
			cfg: Config{
				Endpoint:         "http://vcsa.some-host",
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
		},
		{
			desc: "not http or https",
			cfg: Config{
				Endpoint:         "ws://vcsa.some-host",
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedErr: errors.New("url scheme must be http or https"),
		},
		{
			desc: "unparsable URL",
			cfg: Config{
				Endpoint:         "h" + string(rune(0x7f)),
				ClientConfig:     configtls.ClientConfig{},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedErr: errors.New("unable to parse url"),
		},
		{
			desc: "no username",
			cfg: Config{
				Endpoint:         "https://vcsa.some-host",
				Password:         "otelp",
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedErr: errors.New("username not provided"),
		},
		{
			desc: "no password",
			cfg: Config{
				Endpoint:         "https://vcsa.some-host",
				Username:         "otelu",
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedErr: errors.New("password not provided"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			err := tc.cfg.Validate()
			if tc.expectedErr != nil {
				require.ErrorContains(t, err, tc.expectedErr.Error())
			}
		})
	}
}

func TestLoadConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "").String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))

	expected := factory.CreateDefaultConfig().(*Config)
	expected.Endpoint = "http://vcsa.host.localnet"
	expected.Username = "otelu"
	expected.Password = "${env:VCENTER_PASSWORD}"
	expected.MetricsBuilderConfig = metadata.DefaultMetricsBuilderConfig()
	expected.Metrics.VcenterHostCPUUtilization.Enabled = false
	expected.CollectionInterval = 5 * time.Minute

	if diff := cmp.Diff(expected, cfg, cmpopts.IgnoreUnexported(metadata.MetricConfig{}), cmpopts.IgnoreUnexported(metadata.ResourceAttributeConfig{})); diff != "" {
		t.Errorf("Config mismatch (-expected +actual):\n%s", diff)
	}
}

func TestScraperGroupsConfig(t *testing.T) {
	tests := []struct {
		name        string
		cfg         *Config
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid scraper group",
			cfg: &Config{
				Endpoint:         "https://vcsa.some-host",
				Username:         "user",
				Password:         "pass",
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
				Scrapers: map[ScraperGroup]ScraperConfig{
					ScraperGroupVSAN: {Enabled: true},
				},
			},
			expectError: false,
		},
		{
			name: "invalid scraper group",
			cfg: &Config{
				Endpoint:         "https://vcsa.some-host",
				Username:         "user",
				Password:         "pass",
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
				Scrapers: map[ScraperGroup]ScraperConfig{
					ScraperGroup("invalid"): {Enabled: true},
				},
			},
			expectError: true,
			errorMsg:    "invalid scraper group",
		},
		{
			name: "multiple scraper groups",
			cfg: &Config{
				Endpoint:         "https://vcsa.some-host",
				Username:         "user",
				Password:         "pass",
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
				Scrapers: map[ScraperGroup]ScraperConfig{
					ScraperGroupVSAN:      {Enabled: true},
					ScraperGroupCluster:   {Enabled: false},
					ScraperGroupHost:      {Enabled: true},
					ScraperGroupVM:        {Enabled: true},
					ScraperGroupDatastore: {Enabled: false},
				},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.expectError {
				require.Error(t, err)
				if tt.errorMsg != "" {
					require.ErrorContains(t, err, tt.errorMsg)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestIsScraperGroupEnabled(t *testing.T) {
	tests := []struct {
		name           string
		cfg            *Config
		group          ScraperGroup
		expectedResult bool
	}{
		{
			name: "scraper groups not configured - defaults to enabled",
			cfg: &Config{
				Scrapers: nil,
			},
			group:          ScraperGroupVSAN,
			expectedResult: true,
		},
		{
			name: "scraper group not specified - defaults to enabled",
			cfg: &Config{
				Scrapers: map[ScraperGroup]ScraperConfig{
					ScraperGroupCluster: {Enabled: false},
				},
			},
			group:          ScraperGroupVSAN,
			expectedResult: true,
		},
		{
			name: "scraper group explicitly enabled",
			cfg: &Config{
				Scrapers: map[ScraperGroup]ScraperConfig{
					ScraperGroupVSAN: {Enabled: true},
				},
			},
			group:          ScraperGroupVSAN,
			expectedResult: true,
		},
		{
			name: "scraper group explicitly disabled",
			cfg: &Config{
				Scrapers: map[ScraperGroup]ScraperConfig{
					ScraperGroupVSAN: {Enabled: false},
				},
			},
			group:          ScraperGroupVSAN,
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isScraperGroupEnabled(tt.cfg, tt.group)
			require.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestHasEnabledVSANMetrics(t *testing.T) {
	tests := []struct {
		name           string
		cfg            *Config
		expectedResult bool
	}{
		{
			name: "scraper groups not configured - uses backward compatibility",
			cfg: &Config{
				Scrapers: nil,
				MetricsBuilderConfig: metadata.MetricsBuilderConfig{
					Metrics: metadata.MetricsConfig{
						VcenterClusterVsanOperations: metadata.MetricConfig{Enabled: true},
					},
				},
			},
			expectedResult: true,
		},
		{
			name: "scraper groups configured - vsan enabled",
			cfg: &Config{
				Scrapers: map[ScraperGroup]ScraperConfig{
					ScraperGroupVSAN: {Enabled: true},
				},
				MetricsBuilderConfig: metadata.MetricsBuilderConfig{
					Metrics: metadata.MetricsConfig{
						// Even if individual metrics are disabled, scraper group takes precedence
						VcenterClusterVsanOperations: metadata.MetricConfig{Enabled: false},
					},
				},
			},
			expectedResult: true,
		},
		{
			name: "scraper groups configured - vsan disabled",
			cfg: &Config{
				Scrapers: map[ScraperGroup]ScraperConfig{
					ScraperGroupVSAN: {Enabled: false},
				},
				MetricsBuilderConfig: metadata.MetricsBuilderConfig{
					Metrics: metadata.MetricsConfig{
						// Even if individual metrics are enabled, scraper group takes precedence
						VcenterClusterVsanOperations: metadata.MetricConfig{Enabled: true},
					},
				},
			},
			expectedResult: false,
		},
		{
			name: "scraper groups configured - vsan not specified, defaults to enabled",
			cfg: &Config{
				Scrapers: map[ScraperGroup]ScraperConfig{
					ScraperGroupCluster: {Enabled: false},
				},
				MetricsBuilderConfig: metadata.MetricsBuilderConfig{
					Metrics: metadata.MetricsConfig{
						VcenterClusterVsanOperations: metadata.MetricConfig{Enabled: false},
					},
				},
			},
			expectedResult: true, // Defaults to enabled when not specified
		},
		{
			name: "backward compatibility - no vsan metrics enabled",
			cfg: &Config{
				Scrapers: nil,
				MetricsBuilderConfig: metadata.MetricsBuilderConfig{
					Metrics: metadata.MetricsConfig{
						// All vSAN metrics disabled
						VcenterClusterVsanOperations: metadata.MetricConfig{Enabled: false},
						VcenterClusterVsanThroughput: metadata.MetricConfig{Enabled: false},
						VcenterHostVsanOperations:    metadata.MetricConfig{Enabled: false},
						VcenterVMVsanOperations:      metadata.MetricConfig{Enabled: false},
					},
				},
			},
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scraper := &vcenterMetricScraper{
				config: tt.cfg,
			}
			result := scraper.hasEnabledVSANMetrics()
			require.Equal(t, tt.expectedResult, result)
		})
	}
}
