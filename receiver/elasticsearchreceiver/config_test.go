// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchreceiver

import (
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

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
				require.ErrorIs(t, confmap.Validate(cfg), errPasswordNotSpecified)
			},
		},
		{
			desc: "Username is empty, password specified",
			run: func(t *testing.T) {
				t.Parallel()

				cfg := NewFactory().CreateDefaultConfig().(*Config)
				cfg.Password = "pass"
				require.ErrorIs(t, confmap.Validate(cfg), errUsernameNotSpecified)
			},
		},
		{
			desc: "Username and password are both specified",
			run: func(t *testing.T) {
				t.Parallel()

				cfg := NewFactory().CreateDefaultConfig().(*Config)
				cfg.Username = "user"
				cfg.Password = "pass"
				require.NoError(t, confmap.Validate(cfg))
			},
		},
		{
			desc: "Username and password are both not specified",
			run: func(t *testing.T) {
				t.Parallel()

				cfg := NewFactory().CreateDefaultConfig().(*Config)
				require.NoError(t, confmap.Validate(cfg))
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
			cfg.ClientConfig.Endpoint = testCase.rawURL

			err := confmap.Validate(cfg)

			switch {
			case testCase.expectedErr != nil:
				require.ErrorIs(t, err, testCase.expectedErr)
			case testCase.expectedErrStr != "":
				require.ErrorContains(t, err, testCase.expectedErrStr)
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

	defaultMetrics := metadata.NewDefaultMetricsBuilderConfig()
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
				ControllerConfig: scraperhelper.ControllerConfig{
					CollectionInterval: 2 * time.Minute,
					InitialDelay:       time.Second,
				},
				MetricsBuilderConfig: defaultMetrics,
				Username:             "otel",
				Password:             "password",
				ClientConfig: func() confighttp.ClientConfig {
					client := confighttp.NewDefaultClientConfig()
					client.Timeout = 10000000000
					client.Endpoint = "http://example.com:9200"
					return client
				}(),
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

			assert.NoError(t, confmap.Validate(cfg))
			if diff := cmp.Diff(
				tt.expected,
				cfg,
				// mdatagen gives metric and resource attribute configs an unexported enabledSetByUser,
				// set from parser.IsSet("enabled"), so it is only true on the unmarshaled side:
				// https://github.com/open-telemetry/opentelemetry-collector/blob/e4e58cda0aa6d5d4d275ff12072ae418410e6ae7/cmd/mdatagen/internal/templates/config.go.tmpl#L42-L44
				cmp.FilterPath(
					func(fp cmp.Path) bool {
						return fp.Last().String() == ".enabledSetByUser"
					},
					cmp.Ignore(),
				),
				// Allow go-cmp to read unexported fields instead of panicking on them, so new
				// upstream fields can't break this (https://pkg.go.dev/github.com/google/go-cmp/cmp#Exporter).
				cmp.Exporter(func(reflect.Type) bool { return true })); diff != "" {
				t.Errorf("Config mismatch (-expected +actual):\n%s", diff)
			}
		})
	}
}
