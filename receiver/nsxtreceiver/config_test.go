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
	notValidSchemeClientConfig := confighttp.NewDefaultClientConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	notValidSchemeClientConfig.MaxIdleConns = 0
	notValidSchemeClientConfig.IdleConnTimeout = 0
	notValidSchemeClientConfig.ForceAttemptHTTP2 = false
	notValidSchemeClientConfig.Endpoint = "wss://not-supported-websockets"
	unparsableURLClientConfig := confighttp.NewDefaultClientConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	unparsableURLClientConfig.MaxIdleConns = 0
	unparsableURLClientConfig.IdleConnTimeout = 0
	unparsableURLClientConfig.ForceAttemptHTTP2 = false
	unparsableURLClientConfig.Endpoint = "\x00"
	usernameNotProvidedClientConfig := confighttp.NewDefaultClientConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	usernameNotProvidedClientConfig.MaxIdleConns = 0
	usernameNotProvidedClientConfig.IdleConnTimeout = 0
	usernameNotProvidedClientConfig.ForceAttemptHTTP2 = false
	usernameNotProvidedClientConfig.Endpoint = "http://localhost"
	passwordNotProvidedClientConfig := confighttp.NewDefaultClientConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	passwordNotProvidedClientConfig.MaxIdleConns = 0
	passwordNotProvidedClientConfig.IdleConnTimeout = 0
	passwordNotProvidedClientConfig.ForceAttemptHTTP2 = false
	passwordNotProvidedClientConfig.Endpoint = "http://localhost"
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
				ClientConfig:     notValidSchemeClientConfig,
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedError: errors.New("url scheme must be http or https"),
		},
		{
			desc: "unparsable url",
			cfg: &Config{
				ClientConfig:     unparsableURLClientConfig,
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedError: errors.New("parse"),
		},
		{
			desc: "username not provided",
			cfg: &Config{
				Password:         "password",
				ClientConfig:     usernameNotProvidedClientConfig,
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedError: errors.New("username not provided"),
		},
		{
			desc: "password not provided",
			cfg: &Config{
				Username:         "otelu",
				ClientConfig:     passwordNotProvidedClientConfig,
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
	expected.ClientConfig.Endpoint = "https://nsx-manager-endpoint"
	expected.Username = "admin"
	expected.Password = "${env:NSXT_PASSWORD}"
	expected.ClientConfig.TLS.Insecure = true
	expected.CollectionInterval = time.Minute

	require.Equal(t, expected, cfg)
}
