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
				Garden: GardenConfig{
					Endpoint: "/var/vcap/data/garden/garden.sock",
				},
				CloudFoundry: CfConfig{
					Endpoint:     "https://api.cf.mydomain.com",
					AuthType:     "client_credentials",
					ClientID:     "myclientid",
					ClientSecret: "myclientsecret",
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "all_settings"),
			expected: &Config{
				RefreshInterval:   20 * time.Second,
				CacheSyncInterval: 5 * time.Second,
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
			cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
			require.NoError(t, err)
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()
			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			assert.NoError(t, component.ValidateConfig(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestConfig_Validate(t *testing.T) {
	cases := []struct {
		reason string
		cfg    Config
		msg    string
	}{
		{
			reason: "missing endpoint",
			cfg:    Config{},
			msg:    "config.Endpoint must be specified",
		},
		{
			reason: "missing auth_type",
			cfg: Config{
				CloudFoundry: CfConfig{
					Endpoint: "https://api.cf.mydomain.com",
				},
			},
			msg: "config.AuthType must be specified",
		},
		{
			reason: "unknown auth_type",
			cfg: Config{
				CloudFoundry: CfConfig{
					Endpoint: "https://api.cf.mydomain.com",
					AuthType: "unknown",
				},
			},
			msg: "unknown auth_type: unknown",
		},
		{
			reason: "missing username",
			cfg: Config{
				CloudFoundry: CfConfig{
					Endpoint: "https://api.cf.mydomain.com",
					AuthType: AuthTypeUserPass,
				},
			},
			msg: fieldError(AuthTypeUserPass, "username").Error(),
		},
		{
			reason: "missing clientID",
			cfg: Config{
				CloudFoundry: CfConfig{
					Endpoint: "https://api.cf.mydomain.com",
					AuthType: AuthTypeClientCredentials,
				},
			},
			msg: fieldError(AuthTypeClientCredentials, "client_id").Error(),
		},
		{
			reason: "missing AccessToken",
			cfg: Config{
				CloudFoundry: CfConfig{
					Endpoint: "https://api.cf.mydomain.com",
					AuthType: AuthTypeToken,
				},
			},
			msg: fieldError(AuthTypeToken, "access_token").Error(),
		},
	}

	for _, tCase := range cases {
		t.Run(tCase.reason, func(t *testing.T) {
			err := tCase.cfg.Validate()
			require.EqualError(t, err, tCase.msg)
		})
	}
}
