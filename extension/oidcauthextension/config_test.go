// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oidcauthextension

import (
	"fmt"
	"os"
	"testing"

	"github.com/go-jose/go-jose/v4/jose-util/generator"
	"github.com/stretchr/testify/require"
)

func TestLegacyConfigMapping(t *testing.T) {
	config := &Config{
		IssuerURL:            "https://example.com",
		Audience:             "client-id",
		IgnoreAudience:       true,
		IssuerCAPath:         "/etc/ca.crt",
		UsernameClaim:        "user",
		GroupsClaim:          "groups",
		SupportedSigningAlgs: []string{"RS256"},
	}

	providers := config.getProviderConfigs()
	require.Len(t, providers, 1)
	require.Equal(t, config.IssuerURL, providers[0].IssuerURL)
	require.Equal(t, config.Audience, providers[0].Audience)
	require.Equal(t, config.IgnoreAudience, providers[0].IgnoreAudience)
	require.Equal(t, config.IssuerCAPath, providers[0].IssuerCAPath)
	require.Equal(t, config.UsernameClaim, providers[0].UsernameClaim)
	require.Equal(t, config.GroupsClaim, providers[0].GroupsClaim)
	require.Equal(t, config.SupportedSigningAlgs, providers[0].SupportedSigningAlgs)
}

func TestDuplicateIssuers(t *testing.T) {
	config := &Config{
		Attribute: "authorization",
		Providers: []ProviderCfg{
			{
				IssuerURL: "https://example.com",
				Audience:  "https://example.com",
			},
			{
				IssuerURL: "https://example.com",
				Audience:  "https://example.com",
			},
		},
	}
	require.Error(t, config.Validate())
}

func TestPublicKeysFile(t *testing.T) {
	type testCase struct {
		name        string
		expectErr   string
		fileBuilder func(t *testing.T) string
	}

	testCases := []testCase{
		{
			name:      "missing file",
			expectErr: "could not read file",
			fileBuilder: func(*testing.T) string {
				return "/nonexistent/jwks.json"
			},
		},
		{
			name:      "invalid json",
			expectErr: "failed to parse JWKS",
			fileBuilder: func(t *testing.T) string {
				tmpFile, err := os.CreateTemp(t.TempDir(), "invalid-jwks-*.json")
				require.NoError(t, err)

				_, err = tmpFile.WriteString("not valid json")
				require.NoError(t, err)
				tmpFile.Close()

				return tmpFile.Name()
			},
		},
		{
			name:      "no keys",
			expectErr: errNoSupportedKeys.Error(),
			fileBuilder: func(t *testing.T) string {
				return createJWKSFile(t, []any{})
			},
		},
		{
			name:      "unsupported key type",
			expectErr: errNoSupportedKeys.Error(),
			fileBuilder: func(t *testing.T) string {
				// go-jose interprets []byte as a symmetric key, which is a valid JWK but invalid for our purposes.
				return createJWKSFile(t, []any{[]byte{}})
			},
		},
	}

	for _, algs := range supportedAlgorithms {
		for _, a := range algs {
			testCases = append(testCases, testCase{
				name: fmt.Sprintf("valid config %s", a),
				fileBuilder: func(t *testing.T) string {
					pubKey, _, err := generator.NewSigningKey(a, 0)
					require.NoError(t, err)
					return createJWKSFile(t, []any{pubKey})
				},
			})
		}
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			jwksFile := tc.fileBuilder(t)

			config := &Config{
				Providers: []ProviderCfg{
					{
						IssuerURL:      "https://example.com",
						Audience:       "https://example.com",
						PublicKeysFile: jwksFile,
					},
				},
			}

			if tc.expectErr != "" {
				err := config.Validate()
				require.ErrorContains(t, err, tc.expectErr)
			} else {
				require.NoError(t, config.Validate())
			}
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	require.Equal(t, "authorization", cfg.Attribute)
	require.Equal(t, []string{"RS256"}, cfg.SupportedSigningAlgs)
	// Validate should NOT fail just because of the default SupportedSigningAlgs
	// if nothing else is set (no providers).
	require.NoError(t, cfg.Validate())
}
