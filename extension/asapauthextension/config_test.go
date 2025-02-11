// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package asapauthextension

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/asapauthextension/internal/metadata"
)

// Test keys. Not for use anywhere but these tests.
const (
	privateKey = configopaque.String("data:application/pkcs8;kid=test;base64,MIIBUwIBADANBgkqhkiG9w0BAQEFAASCAT0wggE5AgEAAkEA0ZPr5JeyVDoB8RyZqQsx6qUD+9gMFg1/0hgdAvmytWBMXQJYdwkK2dFJwwZcWJVhJGcOJBDfB/8tcbdJd34KZQIDAQABAkBZD20tJTHJDSWKGsdJyNIbjqhUu4jXTkFFPK4Hd6jz3gV3fFvGnaolsD5Bt50dTXAiSCpFNSb9M9GY6XUAAdlBAiEA6MccfdZRfVapxKtAZbjXuAgMvnPtTvkVmwvhWLT5Wy0CIQDmfE8Et/pou0Jl6eM0eniT8/8oRzBWgy9ejDGfj86PGQIgWePqIL4OofRBgu0O5TlINI0HPtTNo12U9lbUIslgMdECICXT2RQpLcvqj+cyD7wZLZj6vrHZnTFVrnyR/cL2UyxhAiBswe/MCcD7T7J4QkNrCG+ceQGypc7LsxlIxQuKh5GWYA==")
	publicKey  = `-----BEGIN PUBLIC KEY-----
MFwwDQYJKoZIhvcNAQEBBQADSwAwSAJBANGT6+SXslQ6AfEcmakLMeqlA/vYDBYN
f9IYHQL5srVgTF0CWHcJCtnRScMGXFiVYSRnDiQQ3wf/LXG3SXd+CmUCAwEAAQ==
-----END PUBLIC KEY-----`
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		id          component.ID
		expected    component.Config
		expectedErr error
	}{
		{
			id: component.NewID(metadata.Type),
			expected: &Config{
				TTL:        60 * time.Second,
				Audience:   []string{"test_service1", "test_service2"},
				Issuer:     "test_issuer",
				KeyID:      "test_issuer/test_kid",
				PrivateKey: privateKey,
			},
		},
		{
			id:          component.NewIDWithName(metadata.Type, "missingkeyid"),
			expectedErr: errNoKeyIDProvided,
		},
		{
			id:          component.NewIDWithName(metadata.Type, "missingissuer"),
			expectedErr: errNoIssuerProvided,
		},
		{
			id:          component.NewIDWithName(metadata.Type, "missingaudience"),
			expectedErr: errNoAudienceProvided,
		},
		{
			id:          component.NewIDWithName(metadata.Type, "missingpk"),
			expectedErr: errNoPrivateKeyProvided,
		},
	}
	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
			require.NoError(t, err)
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()
			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))
			if tt.expectedErr != nil {
				assert.ErrorIs(t, xconfmap.Validate(cfg), tt.expectedErr)
				return
			}
			assert.NoError(t, xconfmap.Validate(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}
