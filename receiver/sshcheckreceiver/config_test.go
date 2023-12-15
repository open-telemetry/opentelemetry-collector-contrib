// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sshcheckreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sshcheckreceiver"

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sshcheckreceiver/internal/configssh"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sshcheckreceiver/internal/metadata"
)

// check that OTel Collector patterns are implemented
func TestCheckConfig(t *testing.T) {
	t.Parallel()
	if err := componenttest.CheckConfigStruct(&Config{}); err != nil {
		t.Fatal(err)
	}
}

// test the validate function for config
func TestValidate(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		desc        string
		cfg         *Config
		expectedErr error
	}{
		{
			desc: "missing password and key_file",
			cfg: &Config{
				SSHClientSettings: configssh.SSHClientSettings{
					Username: "otelu",
					Endpoint: "goodhost:2222",
				},
				ScraperControllerSettings: scraperhelper.NewDefaultScraperControllerSettings(metadata.Type),
			},
			expectedErr: multierr.Combine(errMissingPasswordAndKeyFile),
		},
		{
			desc: "missing username",
			cfg: &Config{
				SSHClientSettings: configssh.SSHClientSettings{
					Endpoint: "goodhost:2222",
					KeyFile:  "/home/.ssh/id_rsa",
				},
				ScraperControllerSettings: scraperhelper.NewDefaultScraperControllerSettings(metadata.Type),
			},
			expectedErr: multierr.Combine(
				errMissingUsername,
			),
		},
		{
			desc: "invalid endpoint",
			cfg: &Config{
				SSHClientSettings: configssh.SSHClientSettings{
					Endpoint: "badendpoint . cuz spaces:2222",
					Username: "otelu",
					Password: "otelp",
				},
				ScraperControllerSettings: scraperhelper.NewDefaultScraperControllerSettings(metadata.Type),
			},
			expectedErr: multierr.Combine(
				errInvalidEndpoint,
			),
		},
		{
			desc: "no error with password",
			cfg: &Config{
				SSHClientSettings: configssh.SSHClientSettings{
					Endpoint: "localhost:2222",
					Username: "otelu",
					Password: "otelp",
				},
				ScraperControllerSettings: scraperhelper.NewDefaultScraperControllerSettings(metadata.Type),
			},
			expectedErr: error(nil),
		},
		{
			desc: "no error with key_file",
			cfg: &Config{
				SSHClientSettings: configssh.SSHClientSettings{
					Endpoint: "localhost:2222",
					Username: "otelu",
					KeyFile:  "/possibly/a_path",
				},
				ScraperControllerSettings: scraperhelper.NewDefaultScraperControllerSettings(metadata.Type),
			},
			expectedErr: error(nil),
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
	// load test config
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	rcvrs, err := cm.Sub("receivers")
	require.NoError(t, err)
	sshconf, err := rcvrs.Sub("sshcheck")
	require.NoError(t, err)
	// unmarshal to receiver config
	actualConfig, ok := NewFactory().CreateDefaultConfig().(*Config)
	require.True(t, ok)
	require.NoError(t, sshconf.Unmarshal(actualConfig))

	// set expected config
	expectedConfig, ok := NewFactory().CreateDefaultConfig().(*Config)
	require.True(t, ok)

	expectedConfig.ScraperControllerSettings = scraperhelper.ScraperControllerSettings{
		InitialDelay:       time.Second,
		CollectionInterval: 10 * time.Second,
	}
	expectedConfig.SSHClientSettings = configssh.SSHClientSettings{
		Endpoint:      "notdefault:1313",
		Username:      "notdefault_username",
		Password:      "notdefault_password",
		KeyFile:       "notdefault/path/keyfile",
		KnownHosts:    "path/to/collector_known_hosts",
		IgnoreHostKey: false,
		Timeout:       10 * time.Second,
	}
	require.Equal(t, expectedConfig, actualConfig)
}
