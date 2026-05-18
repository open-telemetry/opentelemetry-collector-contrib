// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package gcpsecretsmanagerauthextension

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
		Htpasswd:   &HtpasswdSettings{Project: "p", SecretName: "s"},
		ClientAuth: &ClientAuthSettings{Project: "p", SecretName: "s", UsernameKey: "u", PasswordKey: "p"},
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
			Project:     "my-project",
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
		{"missing project", &ClientAuthSettings{SecretName: "s", UsernameKey: "u", PasswordKey: "p"}, errMissingProject},
		{"missing secret_name", &ClientAuthSettings{Project: "p", UsernameKey: "u", PasswordKey: "p"}, errMissingSecretName},
		{"missing username_key", &ClientAuthSettings{Project: "p", SecretName: "s", PasswordKey: "p"}, errMissingUsernameKey},
		{"missing password_key", &ClientAuthSettings{Project: "p", SecretName: "s", UsernameKey: "u"}, errMissingPasswordKey},
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
			Project:    "my-project",
			SecretName: "my-secret",
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
		{"missing project", &HtpasswdSettings{SecretName: "s"}, errMissingProject},
		{"missing secret_name", &HtpasswdSettings{Project: "p"}, errMissingSecretName},
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
			Project:         "p",
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

	sub, err := cm.Sub("client")
	require.NoError(t, err)

	cfg := &Config{}
	require.NoError(t, sub.Unmarshal(cfg))
	require.NoError(t, cfg.Validate())

	assert.Equal(t, "my-gcp-project-123", cfg.ClientAuth.Project)
	assert.Equal(t, "otel-collector-creds", cfg.ClientAuth.SecretName)
	assert.Equal(t, "username", cfg.ClientAuth.UsernameKey)
	assert.Equal(t, "password", cfg.ClientAuth.PasswordKey)
	assert.Equal(t, 5*time.Minute, cfg.ClientAuth.RefreshInterval)
}
