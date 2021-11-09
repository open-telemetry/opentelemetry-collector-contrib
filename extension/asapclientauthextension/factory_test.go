// Copyright  The OpenTelemetry Authors
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

package asapclientauthextension

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
	testKey := "data:application/pkcs8;kid=test;base64,MIIBVAIBADANBgkqhkiG9w0BAQEFAASCAT4wggE6AgEAAkEA5DswDqF/YTq+c7PmMlc3CZcpJppdcvlNviMy4lCjS9FILXO9bjME6kCNfQE/AxgE3yp3rEmZ/j4oEf1jXUz5AQIDAQABAkBl7b0fu6ac8NRf7idPsj3FTbo2IFi94XOECEpQYr0bPWt6pQyoArlgqF8TlZD/H3zjn+y95DLOeZijleZlh67xAiEA80FkGtTfJRbcLBymJxPjlV9+Agj1o11bLLv0IqS2wYUCIQDwMEmc3pZlfpazeJPwvEmX1h3T72V8NQoRleFr7vr0TQIhALeC8GMxjnoricQZhNtcLMfGd4hPfAhXaG4SCTaNbnYFAiAx1QLgzfmMEyh3EdQ3xQjLvLuxheCbVXHCVkNPnmRonQIgYcX1m7kzkJatn5XuMeU8VndbGT66cpRoGY2FNPzvhZI="

	tests := []struct {
		name        string
		settings    *Config
		shouldError bool
		expectedErr error
	}{
		{
			name: "valid_settings",
			settings: &Config{
				KeyId:      "test_issuer/test_kid",
				Issuer:     "test_issuer",
				Audience:   []string{"test_service"},
				Ttl:        60,
				PrivateKey: testKey,
			},
			shouldError: false,
		},
		{
			name: "invalid_settings_should_error",
			settings: &Config{
				KeyId:      "test_issuer/test_kid",
				Issuer:     "test_issuer",
				Audience:   []string{"test_service"},
				Ttl:        60,
				PrivateKey: "data:application/pkcs8;kid=test;base64,INVALIDPEM", // invalid key data
			},
			shouldError: true,
		},
		{
			name: "invalid_settings_should_error2",
			settings: &Config{
				KeyId:      "test_issuer/test_kid",
				Issuer:     "test_issuer",
				Audience:   []string{"test_service"},
				Ttl:        -10, // invalid ttl
				PrivateKey: testKey,
			},
			shouldError: true,
		},
	}

	for _, testcase := range tests {
		t.Run(testcase.name, func(t *testing.T) {
			cfg.KeyId = testcase.settings.KeyId
			cfg.Issuer = testcase.settings.Issuer
			cfg.Audience = testcase.settings.Audience
			cfg.Ttl = testcase.settings.Ttl
			cfg.PrivateKey = testcase.settings.PrivateKey
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
