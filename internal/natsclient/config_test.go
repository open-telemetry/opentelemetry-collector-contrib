// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package natsclient

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/confmap"
)

func TestNewDefaultClientConfig(t *testing.T) {
	cfg := NewDefaultClientConfig()
	assert.Equal(t, "nats://127.0.0.1:4222", cfg.Endpoint)
	assert.False(t, cfg.Pedantic)
	assert.NoError(t, confmap.Validate(&cfg))
}

func TestAuthConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		auth    AuthConfig
		wantErr string
	}{
		{name: "none"},
		{name: "token", auth: AuthConfig{Token: &TokenConfig{Token: "t"}}},
		{
			name:    "incomplete token",
			auth:    AuthConfig{Token: &TokenConfig{}},
			wantErr: "incomplete token auth configuration",
		},
		{
			name:    "incomplete user",
			auth:    AuthConfig{User: &UserConfig{Username: "otel"}},
			wantErr: "incomplete username/password auth configuration",
		},
		{
			name:    "incomplete nkey",
			auth:    AuthConfig{Nkey: &NkeyConfig{PublicKey: "k"}},
			wantErr: "incomplete NKey auth configuration",
		},
		{
			name:    "incomplete nkey jwt",
			auth:    AuthConfig{NkeyJWT: &NkeyJWTConfig{JWT: "j"}},
			wantErr: "incomplete NKey auth (via JWT) configuration",
		},
		{
			name:    "incomplete nkey user file",
			auth:    AuthConfig{NkeyUserFile: &NkeyUserFileConfig{}},
			wantErr: "incomplete NKey auth (via user file) configuration",
		},
		{
			name: "multiple methods",
			auth: AuthConfig{
				Token:        &TokenConfig{Token: "t"},
				NkeyUserFile: &NkeyUserFileConfig{UserFilePath: "/creds"},
			},
			wantErr: "more than one auth method configured",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.auth.Validate()
			if tt.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			assert.ErrorContains(t, err, tt.wantErr)
		})
	}
}
