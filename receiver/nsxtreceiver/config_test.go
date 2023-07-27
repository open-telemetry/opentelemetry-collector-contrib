// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package nsxtreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/nsxtreceiver"

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/nsxtreceiver/internal/metadata"
)

func TestMetricValidation(t *testing.T) {
	defaultConfig := createDefaultConfig().(*Config)
	cases := []struct {
		desc          string
		cfg           *Config
		expectedError error
	}{
		{
			desc:          "default config",
			cfg:           defaultConfig,
			expectedError: errors.New("no manager endpoint was specified"),
		},
		{
			desc: "not valid scheme",
			cfg: &Config{
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: "wss://not-supported-websockets",
				},
				ScraperControllerSettings: scraperhelper.NewDefaultScraperControllerSettings(""),
			},
			expectedError: errors.New("url scheme must be http or https"),
		},
		{
			desc: "unparseable url",
			cfg: &Config{
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: "\x00",
				},
				ScraperControllerSettings: scraperhelper.NewDefaultScraperControllerSettings(""),
			},
			expectedError: errors.New("parse"),
		},
		{
			desc: "username not provided",
			cfg: &Config{
				Password: "password",
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: "http://localhost",
				},
				ScraperControllerSettings: scraperhelper.NewDefaultScraperControllerSettings(""),
			},
			expectedError: errors.New("username not provided"),
		},
		{
			desc: "password not provided",
			cfg: &Config{
				Username: "otelu",
				HTTPClientSettings: confighttp.HTTPClientSettings{
					Endpoint: "http://localhost",
				},
				ScraperControllerSettings: scraperhelper.NewDefaultScraperControllerSettings(""),
			},
			expectedError: errors.New("password not provided"),
		},
	}
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			err := tc.cfg.Validate()
			if tc.expectedError != nil {
				require.ErrorContains(t, err, tc.expectedError.Error())
			} else {
				require.NoError(t, err)
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
	require.NoError(t, component.UnmarshalConfig(sub, cfg))

	expected := factory.CreateDefaultConfig().(*Config)
	expected.Endpoint = "https://nsx-manager-endpoint"
	expected.Username = "admin"
	expected.Password = "${env:NSXT_PASSWORD}"
	expected.TLSSetting.Insecure = true
	expected.CollectionInterval = time.Minute

	require.Equal(t, expected, cfg)
}
