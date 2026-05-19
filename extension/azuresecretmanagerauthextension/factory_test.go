// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azuresecretmanagerauthextension

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.uber.org/zap"
)

func TestFactory_CreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig()
	assert.NotNil(t, cfg)

	c := cfg.(*Config)
	assert.Nil(t, c.Htpasswd)
	assert.Nil(t, c.ClientAuth)
}

func TestFactory_CreateExtension_Client(t *testing.T) {
	cfg := &Config{
		ClientAuth: &ClientAuthSettings{
			KeyVaultURI:     "https://my-vault.vault.azure.net",
			SecretName:      "my-secret",
			UsernameKey:     "username",
			PasswordKey:     "password",
			RefreshInterval: defaultRefreshInterval,
		},
	}

	set := extension.Settings{
		TelemetrySettings: component.TelemetrySettings{Logger: zap.NewNop()},
	}
	ext, err := createExtension(t.Context(), set, cfg)
	require.NoError(t, err)
	assert.NotNil(t, ext)
	_, ok := ext.(*azureSecretsManagerAuthClient)
	assert.True(t, ok)
}

func TestFactory_CreateExtension_Server(t *testing.T) {
	cfg := &Config{
		Htpasswd: &HtpasswdSettings{
			KeyVaultURI:     "https://my-vault.vault.azure.net",
			SecretName:      "my-secret",
			RefreshInterval: defaultRefreshInterval,
		},
	}

	set := extension.Settings{
		TelemetrySettings: component.TelemetrySettings{Logger: zap.NewNop()},
	}
	ext, err := createExtension(t.Context(), set, cfg)
	require.NoError(t, err)
	assert.NotNil(t, ext)
	_, ok := ext.(*azureSecretsManagerAuthServer)
	assert.True(t, ok)
}
