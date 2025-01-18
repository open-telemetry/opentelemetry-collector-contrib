// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package rabbitmqreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/rabbitmqreceiver"

import (
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/rabbitmqreceiver/internal/metadata"
)

func TestValidate(t *testing.T) {
	clientConfigInvalid := confighttp.NewDefaultClientConfig()
	clientConfigInvalid.Endpoint = "invalid://endpoint: 12efg"

	clientConfig := confighttp.NewDefaultClientConfig()
	clientConfig.Endpoint = defaultEndpoint

	testCases := []struct {
		desc        string
		cfg         *Config
		expectedErr error
	}{
		{
			desc: "missing username, password, and invalid endpoint",
			cfg: &Config{
				ClientConfig: clientConfigInvalid,
			},
			expectedErr: errors.Join(
				errMissingUsername,
				errMissingPassword,
				fmt.Errorf("%w: %s", errInvalidEndpoint, `parse "invalid://endpoint: 12efg": invalid port ": 12efg" after host`)),
		},
		{
			desc: "missing password and invalid endpoint",
			cfg: &Config{
				Username:     "otelu",
				ClientConfig: clientConfigInvalid,
			},
			expectedErr: errors.Join(
				errMissingPassword,
				fmt.Errorf("%w: %s", errInvalidEndpoint, `parse "invalid://endpoint: 12efg": invalid port ": 12efg" after host`),
			),
		},
		{
			desc: "missing username and invalid endpoint",
			cfg: &Config{
				Password:     "otelp",
				ClientConfig: clientConfigInvalid,
			},
			expectedErr: errors.Join(
				errMissingUsername,
				fmt.Errorf("%w: %s", errInvalidEndpoint, `parse "invalid://endpoint: 12efg": invalid port ": 12efg" after host`),
			),
		},
		{
			desc: "invalid endpoint",
			cfg: &Config{
				Username:     "otelu",
				Password:     "otelp",
				ClientConfig: clientConfigInvalid,
			},
			expectedErr: errors.Join(
				fmt.Errorf("%w: %s", errInvalidEndpoint, `parse "invalid://endpoint: 12efg": invalid port ": 12efg" after host`),
			),
		},
		{
			desc: "valid config with node metrics enabled",
			cfg: &Config{
				Username:          "otelu",
				Password:          "otelp",
				ClientConfig:      clientConfig,
				EnableNodeMetrics: true,
			},
			expectedErr: nil,
		},
		{
			desc: "valid config with node metrics disabled",
			cfg: &Config{
				Username:          "otelu",
				Password:          "otelp",
				ClientConfig:      clientConfig,
				EnableNodeMetrics: false,
			},
			expectedErr: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			actualErr := tc.cfg.Validate()
			if tc.expectedErr != nil {
				require.EqualError(t, actualErr, tc.expectedErr.Error())
			} else {
				require.NoError(t, actualErr)
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
	expected.Endpoint = "http://localhost:15672"
	expected.Username = "otelu"
	expected.Password = "${env:RABBITMQ_PASSWORD}"
	expected.CollectionInterval = 10 * time.Second
	expected.EnableNodeMetrics = true

	require.Equal(t, expected, cfg)
}
