// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package aerospikereceiver

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/aerospikereceiver/internal/metadata"
)

func TestValidate(t *testing.T) {
	testCases := []struct {
		name     string
		config   *Config
		expected error
	}{
		{
			name: "blank endpoint",
			config: &Config{
				Endpoint:         "",
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expected: errEmptyEndpoint,
		},
		{
			name: "missing port",
			config: &Config{
				Endpoint:         "localhost",
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expected: errBadEndpoint,
		},
		{
			name: "bad endpoint",
			config: &Config{
				Endpoint:         "x;;ef;s;d:::ss:23423423423423423",
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expected: errBadEndpoint,
		},
		{
			name: "missing host",
			config: &Config{
				Endpoint:         ":3001",
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expected: errBadEndpoint,
		},
		{
			name: "negative port",
			config: &Config{
				Endpoint:         "localhost:-2",
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expected: errBadPort,
		},
		{
			name: "bad port",
			config: &Config{
				Endpoint:         "localhost:9999999999999999999",
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expected: errBadPort,
		},
		{
			name: "negative timeout",
			config: &Config{
				Endpoint:         "localhost:3000",
				Timeout:          -1 * time.Second,
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expected: errNegativeTimeout,
		},
		{
			name: "password but no username",
			config: &Config{
				Endpoint:         "localhost:3000",
				Username:         "",
				Password:         "secret",
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expected: errEmptyUsername,
		},
		{
			name: "username but no password",
			config: &Config{
				Endpoint:         "localhost:3000",
				Username:         "ro_user",
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expected: errEmptyPassword,
		},
		{
			name: "bad TLS config",
			config: &Config{
				Endpoint: "localhost:3000",
				TLSName:  "tls1",
				TLS: &configtls.ClientConfig{
					Insecure: false,
					Config: configtls.Config{
						CAFile: "BADCAFILE",
					},
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expected: errFailedTLSLoad,
		},
		{
			name: "empty tls name",
			config: &Config{
				Endpoint: "localhost:3000",
				TLSName:  "",
				TLS: &configtls.ClientConfig{
					Insecure: false,
					Config:   configtls.Config{},
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expected: errEmptyEndpointTLSName,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.config.Validate()
			require.ErrorIs(t, err, tc.expected)
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
	expected.Endpoint = "localhost:3000"
	expected.CollectionInterval = 30 * time.Second

	require.Equal(t, expected, cfg)
}
