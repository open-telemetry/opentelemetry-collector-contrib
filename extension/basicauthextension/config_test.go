// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package basicauthextension

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/basicauthextension/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		id          component.ID
		expected    component.Config
		expectedErr bool
	}{
		{
			id:          component.NewID(metadata.Type),
			expectedErr: true,
		},
		{
			id: component.NewIDWithName(metadata.Type, "server"),
			expected: &Config{
				Htpasswd: &HtpasswdSettings{
					Inline: "username1:password1\nusername2:password2\n",
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "client"),
			expected: &Config{
				ClientAuth: &ClientAuthSettings{
					Username: "username",
					Password: "password",
				},
			},
		},
		{
			id:          component.NewIDWithName(metadata.Type, "both"),
			expectedErr: true,
		},
		{
			id: component.NewIDWithName(metadata.Type, "client_secret_provider"),
			expected: &Config{
				ClientAuth: &ClientAuthSettings{
					SecretProvider: &SecretProviderConfig{
						ID:          component.MustNewID("awssecretsmanagerprovider"),
						UsernameKey: "username",
						PasswordKey: "password",
					},
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "server_secret_provider"),
			expected: &Config{
				Htpasswd: &HtpasswdSettings{
					SecretProvider: &SecretProviderConfig{
						ID: component.MustNewID("awssecretsmanagerprovider"),
					},
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
			if tt.expectedErr {
				assert.Error(t, xconfmap.Validate(cfg))
				return
			}
			assert.NoError(t, xconfmap.Validate(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestValidate_SecretProviderMutualExclusion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cfg  *Config
		err  error
	}{
		{
			name: "client_secret_provider_with_inline",
			cfg: &Config{
				ClientAuth: &ClientAuthSettings{
					Username: "user",
					SecretProvider: &SecretProviderConfig{
						ID:          component.MustNewID("provider"),
						UsernameKey: "u", PasswordKey: "p",
					},
				},
			},
			err: errSecretProviderAndOtherSource,
		},
		{
			name: "client_secret_provider_with_file",
			cfg: &Config{
				ClientAuth: &ClientAuthSettings{
					PasswordFile: "/path",
					SecretProvider: &SecretProviderConfig{
						ID:          component.MustNewID("provider"),
						UsernameKey: "u", PasswordKey: "p",
					},
				},
			},
			err: errSecretProviderAndOtherSource,
		},
		{
			name: "server_secret_provider_with_file",
			cfg: &Config{
				Htpasswd: &HtpasswdSettings{
					File: "/path",
					SecretProvider: &SecretProviderConfig{
						ID: component.MustNewID("provider"),
					},
				},
			},
			err: errSecretProviderAndOtherSource,
		},
		{
			name: "server_secret_provider_with_inline",
			cfg: &Config{
				Htpasswd: &HtpasswdSettings{
					Inline: "user:pass",
					SecretProvider: &SecretProviderConfig{
						ID: component.MustNewID("provider"),
					},
				},
			},
			err: errSecretProviderAndOtherSource,
		},
		{
			name: "client_secret_provider_missing_id",
			cfg: &Config{
				ClientAuth: &ClientAuthSettings{
					SecretProvider: &SecretProviderConfig{
						UsernameKey: "u", PasswordKey: "p",
					},
				},
			},
			err: errSecretProviderMissingID,
		},
		{
			name: "client_secret_provider_missing_keys",
			cfg: &Config{
				ClientAuth: &ClientAuthSettings{
					SecretProvider: &SecretProviderConfig{
						ID: component.MustNewID("provider"),
					},
				},
			},
			err: errSecretProviderMissingKeys,
		},
		{
			name: "server_secret_provider_missing_id",
			cfg: &Config{
				Htpasswd: &HtpasswdSettings{
					SecretProvider: &SecretProviderConfig{},
				},
			},
			err: errSecretProviderMissingID,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.ErrorIs(t, tt.cfg.Validate(), tt.err)
		})
	}
}
