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

package asapauthextension

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/service/servicetest"
)

// Test keys. Not for use anywhere but these tests.
const (
	privateKey = "data:application/pkcs8;kid=test;base64,MIIBUwIBADANBgkqhkiG9w0BAQEFAASCAT0wggE5AgEAAkEA0ZPr5JeyVDoB8RyZqQsx6qUD+9gMFg1/0hgdAvmytWBMXQJYdwkK2dFJwwZcWJVhJGcOJBDfB/8tcbdJd34KZQIDAQABAkBZD20tJTHJDSWKGsdJyNIbjqhUu4jXTkFFPK4Hd6jz3gV3fFvGnaolsD5Bt50dTXAiSCpFNSb9M9GY6XUAAdlBAiEA6MccfdZRfVapxKtAZbjXuAgMvnPtTvkVmwvhWLT5Wy0CIQDmfE8Et/pou0Jl6eM0eniT8/8oRzBWgy9ejDGfj86PGQIgWePqIL4OofRBgu0O5TlINI0HPtTNo12U9lbUIslgMdECICXT2RQpLcvqj+cyD7wZLZj6vrHZnTFVrnyR/cL2UyxhAiBswe/MCcD7T7J4QkNrCG+ceQGypc7LsxlIxQuKh5GWYA=="
	publicKey  = `-----BEGIN PUBLIC KEY-----
MFwwDQYJKoZIhvcNAQEBBQADSwAwSAJBANGT6+SXslQ6AfEcmakLMeqlA/vYDBYN
f9IYHQL5srVgTF0CWHcJCtnRScMGXFiVYSRnDiQQ3wf/LXG3SXd+CmUCAwEAAQ==
-----END PUBLIC KEY-----`
)

func TestLoadConfig(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.NoError(t, err)
	factory := NewFactory()
	factories.Extensions[typeStr] = factory

	cfg, err := servicetest.LoadConfigAndValidate(filepath.Join("testdata", "config.yaml"), factories)
	assert.NotNil(t, cfg)
	assert.NoError(t, err)
	assert.NoError(t, cfg.Validate())

	expected := factory.CreateDefaultConfig().(*Config)
	expected.TTL = 60 * time.Second
	expected.Audience = []string{"test_service1", "test_service2"}
	expected.Issuer = "test_issuer"
	expected.KeyID = "test_issuer/test_kid"
	expected.PrivateKey = privateKey
	ext := cfg.Extensions[config.NewComponentID(typeStr)]
	assert.Equal(t, expected, ext)
}

func TestLoadBadConfig(t *testing.T) {
	t.Parallel()
	factories, err := componenttest.NopFactories()
	require.NoError(t, err)

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
		cfg, err := servicetest.LoadConfig(filepath.Join("testdata", "config_bad.yaml"), factories)
		assert.NoError(t, err)
		extension := cfg.Extensions[config.NewComponentIDWithName(typeStr, tt.configName)]
		verr := extension.Validate()
		require.ErrorIs(t, verr, tt.expectedErr)
	}
}
