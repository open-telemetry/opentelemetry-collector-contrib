// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oauth2clientauthextension

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/extension/extensiontest"
)

func TestCreateDefaultConfig(t *testing.T) {
	// prepare and test
	expected := &Config{
		ExpiryBuffer: 5 * time.Minute,
	}

	// test
	cfg := createDefaultConfig()

	// verify
	assert.Equal(t, expected, cfg)
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestCreate(t *testing.T) {
	cfg := createDefaultConfig().(*Config)

	tests := []struct {
		name        string
		settings    *Config
		shouldError bool
		expectedErr error
	}{
		{
			name: "valid_settings",
			settings: &Config{
				ClientID:     "testclientid",
				ClientSecret: "testsecret",
				TokenURL:     "https://example.com/v1/token",
				Scopes:       []string{"resource.read"},
			},
			shouldError: false,
		},
	}

	for _, testcase := range tests {
		t.Run(testcase.name, func(t *testing.T) {
			cfg.ClientID = testcase.settings.ClientID
			cfg.ClientSecret = testcase.settings.ClientSecret
			cfg.TokenURL = testcase.settings.TokenURL
			cfg.Scopes = testcase.settings.Scopes
			ext, err := createExtension(context.Background(), extensiontest.NewNopSettings(), cfg)
			if testcase.shouldError {
				assert.Error(t, err)
				assert.Nil(t, ext)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, ext)
			}
		})
	}
}

func TestNewFactory(t *testing.T) {
	f := NewFactory()
	assert.NotNil(t, f)
}
