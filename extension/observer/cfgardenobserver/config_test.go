// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cfgardenobserver

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/cfgardenobserver/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		id       component.ID
		expected component.Config
	}{
		{
			id: component.NewID(metadata.Type),
			expected: &Config{
				RefreshInterval:   1 * time.Minute,
				CacheSyncInterval: 5 * time.Minute,
				IncludeAppLabels:  false,
				Garden: GardenConfig{
					Endpoint: "/var/vcap/data/garden/garden.sock",
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "all_settings"),
			expected: &Config{
				RefreshInterval:   20 * time.Second,
				CacheSyncInterval: 5 * time.Second,
				IncludeAppLabels:  true,
				Garden: GardenConfig{
					Endpoint: "/var/vcap/data/garden/custom.sock",
				},
				CloudFoundry: CfConfig{
					Endpoint: "https://api.cf.mydomain.com",
					Auth: CfAuth{
						Type:     "user_pass",
						Username: "myuser",
						Password: "mypass",
					},
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "user_pass"),
			expected: &Config{
				Garden: GardenConfig{
					Endpoint: "/var/vcap/data/garden/garden.sock",
				},
				RefreshInterval:   1 * time.Minute,
				CacheSyncInterval: 5 * time.Minute,
				IncludeAppLabels:  true,
				CloudFoundry: CfConfig{
					Endpoint: "https://api.cf.mydomain.com",
					Auth: CfAuth{
						Type:     "user_pass",
						Username: "myuser",
						Password: "mypass",
					},
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "client_credentials"),
			expected: &Config{
				Garden: GardenConfig{
					Endpoint: "/var/vcap/data/garden/garden.sock",
				},
				RefreshInterval:   1 * time.Minute,
				CacheSyncInterval: 5 * time.Minute,
				IncludeAppLabels:  true,
				CloudFoundry: CfConfig{
					Endpoint: "https://api.cf.mydomain.com",
					Auth: CfAuth{
						Type:         "client_credentials",
						ClientID:     "myclientid",
						ClientSecret: "myclientsecret",
					},
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "token"),
			expected: &Config{
				Garden: GardenConfig{
					Endpoint: "/var/vcap/data/garden/garden.sock",
				},
				RefreshInterval:   1 * time.Minute,
				CacheSyncInterval: 5 * time.Minute,
				IncludeAppLabels:  true,
				CloudFoundry: CfConfig{
					Endpoint: "https://api.cf.mydomain.com",
					Auth: CfAuth{
						Type:         "token",
						AccessToken:  "myaccesstoken",
						RefreshToken: "myrefreshtoken",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			cfg := loadConfig(t, tt.id)
			assert.NoError(t, component.ValidateConfig(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestConfigValidate(t *testing.T) {
	cases := []struct {
		reason string
		cfg    Config
		msg    string
	}{
		{
			reason: "missing endpoint",
			cfg: Config{
				IncludeAppLabels: true,
			},
			msg: "CloudFoundry.Endpoint must be specified when IncludeAppLabels is set to true",
		},
		{
			reason: "missing cloud_foundry.auth.type",
			cfg: Config{
				IncludeAppLabels: true,
				CloudFoundry: CfConfig{
					Endpoint: "https://api.cf.mydomain.com",
				},
			},
			msg: "CloudFoundry.Auth.Type must be specified when IncludeAppLabels is set to true",
		},
		{
			reason: "unknown cloud_foundry.auth.type",
			cfg: Config{
				IncludeAppLabels: true,
				CloudFoundry: CfConfig{
					Endpoint: "https://api.cf.mydomain.com",
					Auth: CfAuth{
						Type: "unknown",
					},
				},
			},
			msg: "configuration option `auth_type` must be set to one of the following values: [user_pass, client_credentials, token]. Specified value: unknown",
		},
		{
			reason: "missing username",
			cfg: Config{
				IncludeAppLabels: true,
				CloudFoundry: CfConfig{
					Endpoint: "https://api.cf.mydomain.com",
					Auth: CfAuth{
						Type: authTypeUserPass,
					},
				},
			},
			msg: fieldError(authTypeUserPass, "username").Error(),
		},
		{
			reason: "missing clientID",
			cfg: Config{
				IncludeAppLabels: true,
				CloudFoundry: CfConfig{
					Endpoint: "https://api.cf.mydomain.com",
					Auth: CfAuth{
						Type: authTypeClientCredentials,
					},
				},
			},
			msg: fieldError(authTypeClientCredentials, "client_id").Error(),
		},
		{
			reason: "missing AccessToken",
			cfg: Config{
				IncludeAppLabels: true,
				CloudFoundry: CfConfig{
					Endpoint: "https://api.cf.mydomain.com",
					Auth: CfAuth{
						Type: authTypeToken,
					},
				},
			},
			msg: fieldError(authTypeToken, "access_token").Error(),
		},
	}

	for _, tCase := range cases {
		t.Run(tCase.reason, func(t *testing.T) {
			err := tCase.cfg.Validate()
			require.EqualError(t, err, tCase.msg)
		})
	}
}

func loadRawConf(tb testing.TB, path string, id component.ID) *confmap.Conf {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", path))
	require.NoError(tb, err)
	sub, err := cm.Sub(id.String())
	require.NoError(tb, err)
	return sub
}

func loadConfig(tb testing.TB, id component.ID) *Config {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	sub := loadRawConf(tb, "config.yaml", id)
	require.NoError(tb, sub.Unmarshal(cfg))
	return cfg.(*Config)
}
