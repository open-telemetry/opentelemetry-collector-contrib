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
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configtest"
)

func TestLoadConfig(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.NoError(t, err)
	factory := NewFactory()
	factories.Extensions[typeStr] = factory

	cfg, err := configtest.LoadConfigAndValidate(path.Join(".", "testdata", "config.yaml"), factories)

	require.NotNil(t, cfg)
	assert.NoError(t, cfg.Validate())

	expected := factory.CreateDefaultConfig().(*Config)
	expected.Ttl = 60
	expected.Audience = []string{"test_service1", "test_service2"}
	expected.Issuer = "test_issuer"
	expected.KeyId = "test_issuer/test_kid"
	expected.PrivateKey = "data:application/pkcs8;kid=test;base64,MIIBVAIBADANBgkqhkiG9w0BAQEFAASCAT4wggE6AgEAAkEA5DswDqF/YTq+c7PmMlc3CZcpJppdcvlNviMy4lCjS9FILXO9bjME6kCNfQE/AxgE3yp3rEmZ/j4oEf1jXUz5AQIDAQABAkBl7b0fu6ac8NRf7idPsj3FTbo2IFi94XOECEpQYr0bPWt6pQyoArlgqF8TlZD/H3zjn+y95DLOeZijleZlh67xAiEA80FkGtTfJRbcLBymJxPjlV9+Agj1o11bLLv0IqS2wYUCIQDwMEmc3pZlfpazeJPwvEmX1h3T72V8NQoRleFr7vr0TQIhALeC8GMxjnoricQZhNtcLMfGd4hPfAhXaG4SCTaNbnYFAiAx1QLgzfmMEyh3EdQ3xQjLvLuxheCbVXHCVkNPnmRonQIgYcX1m7kzkJatn5XuMeU8VndbGT66cpRoGY2FNPzvhZI="

	ext := cfg.Extensions[config.NewComponentID(typeStr)]
	assert.Equal(t, expected, ext)
}

func TestLoadBadConfig(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.NoError(t, err)

	tests := []struct {
		configName  string
		expectedErr error
	}{
		{
			"missingkeyid",
			errNoKeyIDProvided,
		},
		{
			"missingissuer",
			errNoIssuerProvided,
		},
		{
			"missingaudience",
			errNoAudienceProvided,
		},
		{
			"missingpk",
			errNoPrivateKeyProvided,
		},
	}
	for _, tt := range tests {
		factory := NewFactory()
		factories.Extensions[typeStr] = factory
		cfg, _ := configtest.LoadConfig(path.Join(".", "testdata", "config_bad.yaml"), factories)
		extension := cfg.Extensions[config.NewComponentIDWithName(typeStr, tt.configName)]
		verr := extension.Validate()
		require.ErrorIs(t, verr, tt.expectedErr)
	}
}
