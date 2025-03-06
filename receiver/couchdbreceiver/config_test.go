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
	"go.opentelemetry.io/collector/scraper/scraperhelper"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/couchdbreceiver/internal/metadata"
)

func TestValidate(t *testing.T) {
	clientConfigInvalid := confighttp.NewDefaultClientConfig()
	clientConfigInvalid.Endpoint = "http://localhost :5984"

	clientConfig := confighttp.NewDefaultClientConfig()
	clientConfig.Endpoint = "http://localhost:5984"

	testCases := []struct {
		desc        string
		cfg         *Config
		expectedErr error
	}{
		{
			desc: "missing username, password and invalid endpoint",
			cfg: &Config{
				ClientConfig:     clientConfigInvalid,
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
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
				ClientConfig:     clientConfigInvalid,
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
				Username:         "otelu",
			},
			expectedErr: multierr.Combine(
				errMissingPassword,
				fmt.Errorf(errInvalidEndpoint.Error(), "parse \"http://localhost :5984\": invalid character \" \" in host name"),
			),
		},
		{
			desc: "missing username and invalid endpoint",
			cfg: &Config{
				ClientConfig:     clientConfigInvalid,
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
				Password:         "otelp",
			},
			expectedErr: multierr.Combine(
				errMissingUsername,
				fmt.Errorf(errInvalidEndpoint.Error(), "parse \"http://localhost :5984\": invalid character \" \" in host name"),
			),
		},
		{
			desc: "invalid endpoint",
			cfg: &Config{
				Username:         "otel",
				Password:         "otel",
				ClientConfig:     clientConfigInvalid,
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedErr: fmt.Errorf(errInvalidEndpoint.Error(), "parse \"http://localhost :5984\": invalid character \" \" in host name"),
		},
		{
			desc: "no error",
			cfg: &Config{
				Username:         "otel",
				Password:         "otel",
				ClientConfig:     clientConfig,
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
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
	require.NoError(t, sub.Unmarshal(cfg))

	expected := factory.CreateDefaultConfig().(*Config)
	expected.Endpoint = "http://localhost:5984"
	expected.Username = "otelu"
	expected.Password = "${env:COUCHDB_PASSWORD}"
	expected.CollectionInterval = time.Minute

	require.Equal(t, expected, cfg)
}
