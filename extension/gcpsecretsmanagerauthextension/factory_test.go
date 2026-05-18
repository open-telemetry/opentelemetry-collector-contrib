// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package gcpsecretsmanagerauthextension

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.uber.org/zap"
)

func TestFactory_DefaultConfig(t *testing.T) {
	f := NewFactory()
	cfg := f.CreateDefaultConfig()
	require.NotNil(t, cfg)

	c := cfg.(*Config)
	assert.Nil(t, c.Htpasswd)
	assert.Nil(t, c.ClientAuth)
}

func TestFactory_CreateExtension_Client(t *testing.T) {
	f := NewFactory()
	cfg := &Config{
		ClientAuth: &ClientAuthSettings{
			Project:         "my-project",
			SecretName:      "my-secret",
			UsernameKey:     "user",
			PasswordKey:     "pass",
			RefreshInterval: 5 * time.Minute,
		},
	}

	set := extension.Settings{
		ID:                component.NewID(component.MustNewType("gcpsecretsmanagerauth")),
		TelemetrySettings: component.TelemetrySettings{Logger: zap.NewNop()},
	}
	ext, err := f.Create(context.Background(), set, cfg)
	require.NoError(t, err)
	assert.IsType(t, &gcpSecretsManagerAuthClient{}, ext)
}

func TestFactory_CreateExtension_Server(t *testing.T) {
	f := NewFactory()
	cfg := &Config{
		Htpasswd: &HtpasswdSettings{
			Project:         "my-project",
			SecretName:      "my-secret",
			RefreshInterval: 5 * time.Minute,
		},
	}

	set := extension.Settings{
		ID:                component.NewID(component.MustNewType("gcpsecretsmanagerauth")),
		TelemetrySettings: component.TelemetrySettings{Logger: zap.NewNop()},
	}
	ext, err := f.Create(context.Background(), set, cfg)
	require.NoError(t, err)
	assert.IsType(t, &gcpSecretsManagerAuthServer{}, ext)
}
