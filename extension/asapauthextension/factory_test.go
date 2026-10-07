// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package asapauthextension

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/extension/extensiontest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/asapauthextension/internal/metadata"
)

func TestCreateDefaultConfig(t *testing.T) {
	// prepare and test
	expected := &Config{}

	// test
	cfg := createDefaultConfig()

	// verify
	assert.Equal(t, expected, cfg)
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestNewFactory(t *testing.T) {
	f := NewFactory()
	assert.NotNil(t, f)
}

func TestCreate(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	testKey := privateKey

	tests := []struct {
		name        string
		settings    *Config
		shouldError bool
		expectedErr error
	}{
		{
			name: "valid_settings",
			settings: &Config{
				KeyID:      "test_issuer/test_kid",
				Issuer:     "test_issuer",
				Audience:   []string{"test_service"},
				TTL:        60,
				PrivateKey: testKey,
			},
			shouldError: false,
		},
		{
			name: "invalid_settings_should_error",
			settings: &Config{
				KeyID:      "test_issuer/test_kid",
				Issuer:     "test_issuer",
				Audience:   []string{"test_service"},
				TTL:        60,
				PrivateKey: "data:application/pkcs8;kid=test;base64,INVALIDPEM", // invalid key data
			},
			shouldError: true,
		},
	}

	for _, testcase := range tests {
		t.Run(testcase.name, func(t *testing.T) {
			cfg.KeyID = testcase.settings.KeyID
			cfg.Issuer = testcase.settings.Issuer
			cfg.Audience = testcase.settings.Audience
			cfg.TTL = testcase.settings.TTL
			cfg.PrivateKey = testcase.settings.PrivateKey

			// validate extension creation
			ext, err := createExtension(t.Context(), extensiontest.NewNopSettings(extensiontest.NopType), cfg)
			if testcase.shouldError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, ext)
			}
		})
	}
}

func TestFactoryTypeAlias(t *testing.T) {
	factory := NewFactory()
	require.Equal(t, component.MustNewType("asap_client"), factory.Type())

	for _, typ := range []component.Type{metadata.Type, component.MustNewType("asapclient")} {
		t.Run(typ.String(), func(t *testing.T) {
			cfg := factory.CreateDefaultConfig().(*Config)
			cfg.KeyID = "test_issuer/test_kid"
			cfg.Issuer = "test_issuer"
			cfg.Audience = []string{"test_service"}
			cfg.PrivateKey = privateKey
			comp, err := factory.Create(t.Context(), extensiontest.NewNopSettings(typ), cfg)
			require.NoError(t, err)
			require.NotNil(t, comp)
			require.NoError(t, comp.Shutdown(t.Context()))
		})
	}
}
