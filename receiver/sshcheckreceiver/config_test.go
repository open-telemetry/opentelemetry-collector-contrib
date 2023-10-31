// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sshcheckreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sshcheckreceiver"

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
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
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "").String())
	require.NoError(t, err)
	require.NoError(t, component.UnmarshalConfig(sub, cfg))

	expected := factory.CreateDefaultConfig().(*Config)
	expected.Endpoint = "notdefault:1313"
	expected.Username = "notdefault_username"
	expected.Password = "notdefault_password"
	expected.KeyFile = "notdefault/path/keyfile"
	expected.CollectionInterval = 10 * time.Second
	expected.KnownHosts = "path/to/colletor_known_hosts"
	expected.IgnoreHostKey = false

	require.Equal(t, expected, cfg)
}
