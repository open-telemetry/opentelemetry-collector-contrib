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
					AuthType: "user_pass",
					Username: "myuser",
					Password: "mypass",
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
					AuthType: "user_pass",
					Username: "myuser",
					Password: "mypass",
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
					Endpoint:     "https://api.cf.mydomain.com",
					AuthType:     "client_credentials",
					ClientID:     "myclientid",
					ClientSecret: "myclientsecret",
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
					Endpoint:     "https://api.cf.mydomain.com",
					AuthType:     "token",
					AccessToken:  "myaccesstoken",
					RefreshToken: "myrefreshtoken",
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
			msg: "config.Endpoint must be specified when include_app_labels is set to true",
		},
		{
			reason: "missing auth_type",
			cfg: Config{
				IncludeAppLabels: true,
				CloudFoundry: CfConfig{
					Endpoint: "https://api.cf.mydomain.com",
				},
			},
			msg: "config.authType must be specified when include_app_labels is set to true",
		},
		{
			reason: "unknown auth_type",
			cfg: Config{
				IncludeAppLabels: true,
				CloudFoundry: CfConfig{
					Endpoint: "https://api.cf.mydomain.com",
					AuthType: "unknown",
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
					AuthType: authTypeUserPass,
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
					AuthType: authTypeClientCredentials,
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
					AuthType: authTypeToken,
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

func loadConf(t testing.TB, path string, id component.ID) *confmap.Conf {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", path))
	require.NoError(t, err)
	sub, err := cm.Sub(id.String())
	require.NoError(t, err)
	return sub
}

func loadConfig(t testing.TB, id component.ID) *Config {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	sub := loadConf(t, "config.yaml", id)
	require.NoError(t, sub.Unmarshal(cfg))
	return cfg.(*Config)
}
