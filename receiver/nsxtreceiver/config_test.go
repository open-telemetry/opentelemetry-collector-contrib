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
	"go.opentelemetry.io/collector/scraper/scraperhelper"

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
				ClientConfig: confighttp.ClientConfig{
					Endpoint: "wss://not-supported-websockets",
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedError: errors.New("url scheme must be http or https"),
		},
		{
			desc: "unparsable url",
			cfg: &Config{
				ClientConfig: confighttp.ClientConfig{
					Endpoint: "\x00",
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedError: errors.New("parse"),
		},
		{
			desc: "username not provided",
			cfg: &Config{
				Password: "password",
				ClientConfig: confighttp.ClientConfig{
					Endpoint: "http://localhost",
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedError: errors.New("username not provided"),
		},
		{
			desc: "password not provided",
			cfg: &Config{
				Username: "otelu",
				ClientConfig: confighttp.ClientConfig{
					Endpoint: "http://localhost",
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
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
	require.NoError(t, sub.Unmarshal(cfg))

	expected := factory.CreateDefaultConfig().(*Config)
	expected.Endpoint = "https://nsx-manager-endpoint"
	expected.Username = "admin"
	expected.Password = "${env:NSXT_PASSWORD}"
	expected.TLSSetting.Insecure = true
	expected.CollectionInterval = time.Minute

	require.Equal(t, expected, cfg)
}
