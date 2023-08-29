// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package couchdbreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/couchdbreceiver"

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/couchdbreceiver/internal/metadata"
)

func TestValidate(t *testing.T) {
	testCases := []struct {
		desc        string
		cfg         *Config
		expectedErr error
	}{
		{
			desc: "missing username, password and invalid endpoint",
			cfg: &Config{
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: "http://localhost :5984",
				},
				ScraperControllerSettings: scraperhelper.NewDefaultScraperControllerSettings(metadata.Type),
			},
			expectedErr: multierr.Combine(
				errMissingUsername,
				errMissingPassword,
				fmt.Errorf(errInvalidEndpoint.Error(), "parse \"http://localhost :5984\": invalid character \" \" in host name"),
			),
		},
		{
			desc: "missing password and invalid endpoint",
			cfg: &Config{
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: "http://localhost :5984",
				},
				ScraperControllerSettings: scraperhelper.NewDefaultScraperControllerSettings(metadata.Type),
				Username:                  "otelu",
			},
			expectedErr: multierr.Combine(
				errMissingPassword,
				fmt.Errorf(errInvalidEndpoint.Error(), "parse \"http://localhost :5984\": invalid character \" \" in host name"),
			),
		},
		{
			desc: "missing username and invalid endpoint",
			cfg: &Config{
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: "http://localhost :5984",
				},
				ScraperControllerSettings: scraperhelper.NewDefaultScraperControllerSettings(metadata.Type),
				Password:                  "otelp",
			},
			expectedErr: multierr.Combine(
				errMissingUsername,
				fmt.Errorf(errInvalidEndpoint.Error(), "parse \"http://localhost :5984\": invalid character \" \" in host name"),
			),
		},
		{
			desc: "invalid endpoint",
			cfg: &Config{
				Username: "otel",
				Password: "otel",
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: "http://localhost :5984",
				},
				ScraperControllerSettings: scraperhelper.NewDefaultScraperControllerSettings(metadata.Type),
			},
			expectedErr: fmt.Errorf(errInvalidEndpoint.Error(), "parse \"http://localhost :5984\": invalid character \" \" in host name"),
		},
		{
			desc: "no error",
			cfg: &Config{
				Username: "otel",
				Password: "otel",
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: "http://localhost:5984",
				},
				ScraperControllerSettings: scraperhelper.NewDefaultScraperControllerSettings(metadata.Type),
			},
			expectedErr: nil,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			actualErr := tc.cfg.Validate()
			require.Equal(t, tc.expectedErr, actualErr)
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
	require.NoError(t, component.UnmarshalConfig(sub, cfg))

	expected := factory.CreateDefaultConfig().(*Config)
	expected.Endpoint = "http://localhost:5984"
	expected.Username = "otelu"
	expected.Password = "${env:COUCHDB_PASSWORD}"
	expected.CollectionInterval = time.Minute

	require.Equal(t, expected, cfg)
}
