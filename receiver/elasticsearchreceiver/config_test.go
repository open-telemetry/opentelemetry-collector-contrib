// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchreceiver

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/elasticsearchreceiver/internal/metadata"
)

func TestValidateCredentials(t *testing.T) {
	testCases := []struct {
		desc string
		run  func(t *testing.T)
	}{
		{
			desc: "Password is empty, username specified",
			run: func(t *testing.T) {
				t.Parallel()

				cfg := NewFactory().CreateDefaultConfig().(*Config)
				cfg.Username = "user"
				require.ErrorIs(t, component.ValidateConfig(cfg), errPasswordNotSpecified)
			},
		},
		{
			desc: "Username is empty, password specified",
			run: func(t *testing.T) {
				t.Parallel()

				cfg := NewFactory().CreateDefaultConfig().(*Config)
				cfg.Password = "pass"
				require.ErrorIs(t, component.ValidateConfig(cfg), errUsernameNotSpecified)
			},
		},
		{
			desc: "Username and password are both specified",
			run: func(t *testing.T) {
				t.Parallel()

				cfg := NewFactory().CreateDefaultConfig().(*Config)
				cfg.Username = "user"
				cfg.Password = "pass"
				require.NoError(t, component.ValidateConfig(cfg))
			},
		},
		{
			desc: "Username and password are both not specified",
			run: func(t *testing.T) {
				t.Parallel()

				cfg := NewFactory().CreateDefaultConfig().(*Config)
				require.NoError(t, component.ValidateConfig(cfg))
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.desc, testCase.run)
	}
}

func TestValidateEndpoint(t *testing.T) {
	testCases := []struct {
		desc           string
		rawURL         string
		expectedErr    error
		expectedErrStr string
	}{
		{
			desc:   "Default endpoint",
			rawURL: defaultEndpoint,
		},
		{
			desc:        "Empty endpoint",
			rawURL:      "",
			expectedErr: errEmptyEndpoint,
		},
		{
			desc:        "Endpoint with no scheme",
			rawURL:      "localhost",
			expectedErr: errEndpointBadScheme,
		},
		{
			desc:        "Endpoint with unusable scheme",
			rawURL:      "tcp://192.168.1.0",
			expectedErr: errEndpointBadScheme,
		},
		{
			desc:           "URL with control characters",
			rawURL:         "http://\x00",
			expectedErrStr: "invalid endpoint",
		},
		{
			desc:   "Https url",
			rawURL: "https://example.com",
		},
		{
			desc:   "IP + port URL",
			rawURL: "https://192.168.1.1:9200",
		},
	}
	for i := range testCases {
		// Explicitly capture the testCase in this scope instead of using a loop variable;
		// The loop variable will mutate, and all tests will run with the parameters of the last case,
		// if we don't do this
		testCase := testCases[i]
		t.Run(testCase.desc, func(t *testing.T) {
			t.Parallel()

			cfg := NewFactory().CreateDefaultConfig().(*Config)
			cfg.Endpoint = testCase.rawURL

			err := component.ValidateConfig(cfg)

			switch {
			case testCase.expectedErr != nil:
				require.ErrorIs(t, err, testCase.expectedErr)
			case testCase.expectedErrStr != "":
				require.Error(t, err)
				require.Contains(t, err.Error(), testCase.expectedErrStr)
			default:
				require.NoError(t, err)
			}
		})
	}
}

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	defaultMetrics := metadata.DefaultMetricsBuilderConfig()
	defaultMetrics.Metrics.ElasticsearchNodeFsDiskAvailable.Enabled = false
	tests := []struct {
		id       component.ID
		expected component.Config
	}{
		{
			id:       component.NewIDWithName(metadata.Type, "defaults"),
			expected: createDefaultConfig(),
		},
		{
			id: component.NewIDWithName(metadata.Type, ""),
			expected: &Config{
				SkipClusterMetrics: true,
				Nodes:              []string{"_local"},
				Indices:            []string{".geoip_databases"},
				ScraperControllerSettings: scraperhelper.ScraperControllerSettings{
					CollectionInterval: 2 * time.Minute,
				},
				MetricsBuilderConfig: defaultMetrics,
				Username:             "otel",
				Password:             "password",
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Timeout:  10000000000,
					Endpoint: "http://example.com:9200",
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
			require.NoError(t, component.UnmarshalConfig(sub, cfg))

			assert.NoError(t, component.ValidateConfig(cfg))
			if diff := cmp.Diff(tt.expected, cfg, cmpopts.IgnoreUnexported(metadata.MetricConfig{})); diff != "" {
				t.Errorf("Config mismatch (-expected +actual):\n%s", diff)
			}
		})
	}
}
