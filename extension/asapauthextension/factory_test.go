// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package asapauthextension

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configtest"
)

func TestCreateDefaultConfig(t *testing.T) {
	// prepare and test
	expected := &Config{
		ExtensionSettings: config.NewExtensionSettings(config.NewComponentID(typeStr)),
	}

	// test
	cfg := createDefaultConfig()

	// verify
	assert.Equal(t, expected, cfg)
	assert.NoError(t, configtest.CheckConfigStruct(cfg))
}

func TestNewFactory(t *testing.T) {
	f := NewFactory()
	assert.NotNil(t, f)
}

func TestCreateExtension(t *testing.T) {
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
			ext, err := createExtension(context.Background(), componenttest.NewNopExtensionCreateSettings(), cfg)
			if testcase.shouldError {
				assert.Error(t, err)
			} else {
				assert.Nil(t, err)
				assert.NotNil(t, ext)
			}
		})
	}
}
