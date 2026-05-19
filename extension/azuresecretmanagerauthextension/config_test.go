// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuresecretmanagerauthextension

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap/confmaptest"
)

func TestValidate_BothSet(t *testing.T) {
	cfg := &Config{
		Htpasswd:   &HtpasswdSettings{KeyVaultURI: "https://v.vault.azure.net", SecretName: "s"},
		ClientAuth: &ClientAuthSettings{KeyVaultURI: "https://v.vault.azure.net", SecretName: "s", UsernameKey: "u", PasswordKey: "p"},
	}
	assert.ErrorIs(t, cfg.Validate(), errMultipleAuthenticators)
}

func TestValidate_NeitherSet(t *testing.T) {
	cfg := &Config{}
	assert.ErrorIs(t, cfg.Validate(), errNoCredentialSource)
}

func TestValidate_ClientAuth_Valid(t *testing.T) {
	cfg := &Config{
		ClientAuth: &ClientAuthSettings{
			KeyVaultURI: "https://my-vault.vault.azure.net",
			SecretName:  "my-secret",
			UsernameKey: "user",
			PasswordKey: "pass",
		},
	}
	require.NoError(t, cfg.Validate())
	assert.Equal(t, defaultRefreshInterval, cfg.ClientAuth.RefreshInterval)
}

func TestValidate_ClientAuth_MissingFields(t *testing.T) {
	tests := []struct {
		name string
		cfg  *ClientAuthSettings
		err  error
	}{
		{"missing key_vault_uri", &ClientAuthSettings{SecretName: "s", UsernameKey: "u", PasswordKey: "p"}, errMissingKeyVaultURI},
		{"missing secret_name", &ClientAuthSettings{KeyVaultURI: "https://v.vault.azure.net", UsernameKey: "u", PasswordKey: "p"}, errMissingSecretName},
		{"missing username_key", &ClientAuthSettings{KeyVaultURI: "https://v.vault.azure.net", SecretName: "s", PasswordKey: "p"}, errMissingUsernameKey},
		{"missing password_key", &ClientAuthSettings{KeyVaultURI: "https://v.vault.azure.net", SecretName: "s", UsernameKey: "u"}, errMissingPasswordKey},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{ClientAuth: tt.cfg}
			assert.ErrorIs(t, cfg.Validate(), tt.err)
		})
	}
}

func TestValidate_Htpasswd_Valid(t *testing.T) {
	cfg := &Config{
		Htpasswd: &HtpasswdSettings{
			KeyVaultURI: "https://my-vault.vault.azure.net",
			SecretName:  "my-secret",
		},
	}
	require.NoError(t, cfg.Validate())
	assert.Equal(t, defaultRefreshInterval, cfg.Htpasswd.RefreshInterval)
}

func TestValidate_Htpasswd_MissingFields(t *testing.T) {
	tests := []struct {
		name string
		cfg  *HtpasswdSettings
		err  error
	}{
		{"missing key_vault_uri", &HtpasswdSettings{SecretName: "s"}, errMissingKeyVaultURI},
		{"missing secret_name", &HtpasswdSettings{KeyVaultURI: "https://v.vault.azure.net"}, errMissingSecretName},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{Htpasswd: tt.cfg}
			assert.ErrorIs(t, cfg.Validate(), tt.err)
		})
	}
}

func TestValidate_NegativeRefreshInterval(t *testing.T) {
	cfg := &Config{
		ClientAuth: &ClientAuthSettings{
			KeyVaultURI:     "https://v.vault.azure.net",
			SecretName:      "s",
			UsernameKey:     "u",
			PasswordKey:     "p",
			RefreshInterval: -1 * time.Second,
		},
	}
	assert.ErrorIs(t, cfg.Validate(), errNegativeRefreshInterval)
}

func TestLoadConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id       string
		expected *Config
	}{
		{
			id: "azuresecretsmanagerauth/client",
			expected: &Config{
				ClientAuth: &ClientAuthSettings{
					KeyVaultURI:     "https://my-vault.vault.azure.net",
					SecretName:      "otel-collector-creds",
					UsernameKey:     "username",
					PasswordKey:     "password",
					RefreshInterval: 5 * time.Minute,
				},
			},
		},
		{
			id: "azuresecretsmanagerauth/server",
			expected: &Config{
				Htpasswd: &HtpasswdSettings{
					KeyVaultURI:     "https://my-vault.vault.azure.net",
					SecretName:      "otel-collector-htpasswd",
					RefreshInterval: 5 * time.Minute,
				},
			},
		},
		{
			id: "azuresecretsmanagerauth/server_json",
			expected: &Config{
				Htpasswd: &HtpasswdSettings{
					KeyVaultURI:     "https://my-vault.vault.azure.net",
					SecretName:      "otel-collector-htpasswd-json",
					ValueKey:        "htpasswd_content",
					RefreshInterval: 5 * time.Minute,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			sub, err := cm.Sub(tt.id)
			require.NoError(t, err)

			cfg := &Config{}
			require.NoError(t, sub.Unmarshal(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}
