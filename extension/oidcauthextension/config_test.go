// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oidcauthextension

import (
	"fmt"
	"os"
	"testing"

	"github.com/go-jose/go-jose/v4/jose-util/generator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

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

func TestIgnoreIssuers(t *testing.T) {
	type testCase struct {
		name      string
		config    *Config
		expectErr bool
	}

	testCases := []testCase{
		{
			name: "One provider ignore issuer true",
			config: &Config{
				Attribute: "authorization",
				Providers: []ProviderCfg{
					{
						IssuerURL:    "https://example.com",
						Audience:     "https://example.com",
						IgnoreIssuer: true,
					},
				},
			},
			expectErr: false,
		},
		{
			name: "Multi provider ignore issuer true",
			config: &Config{
				Attribute: "authorization",
				Providers: []ProviderCfg{
					{
						IssuerURL:    "https://issuer1.example.com",
						Audience:     "https://example.com",
						IgnoreIssuer: true,
					},
					{
						IssuerURL:    "https://issuer2.example.com",
						Audience:     "https://example.com",
						IgnoreIssuer: false,
					},
				},
			},
			expectErr: true,
		},
		{
			name: "Multi provider ignore issuer all true",
			config: &Config{
				Attribute: "authorization",
				Providers: []ProviderCfg{
					{
						IssuerURL:    "https://issuer1.example.com",
						Audience:     "https://example.com",
						IgnoreIssuer: true,
					},
					{
						IssuerURL:    "https://issuer2.example.com",
						Audience:     "https://example.com",
						IgnoreIssuer: true,
					},
				},
			},
			expectErr: true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.expectErr {
				require.ErrorIs(t, tc.config.Validate(), errIgnoreIssuerMultiProvider)
			} else {
				require.NoError(t, tc.config.Validate())
			}
		})
	}
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

func TestLegacySingleProviderMapping(t *testing.T) {
	config := &Config{
		IssuerURL:      "https://issuer.example.com",
		Audience:       "test-audience",
		IgnoreAudience: false,
		IssuerCAPath:   "/path/to/ca.pem",
		UsernameClaim:  "preferred_username",
		GroupsClaim:    "roles",
	}

	require.NoError(t, config.Validate())
	require.Len(t, config.Providers, 1)
	assert.Equal(t, "https://issuer.example.com", config.Providers[0].IssuerURL)
	assert.Equal(t, "test-audience", config.Providers[0].Audience)
	assert.False(t, config.Providers[0].IgnoreAudience)
	assert.Equal(t, "/path/to/ca.pem", config.Providers[0].IssuerCAPath)
	assert.Equal(t, "preferred_username", config.Providers[0].UsernameClaim)
	assert.Equal(t, "roles", config.Providers[0].GroupsClaim)

	// Legacy fields on config struct should be cleared after mapping
	assert.Empty(t, config.IssuerURL)
	assert.Empty(t, config.Audience)
	assert.Empty(t, config.IssuerCAPath)
	assert.Empty(t, config.UsernameClaim)
	assert.Empty(t, config.GroupsClaim)
	assert.True(t, config.hasLegacySingleProvider())
}

func TestLegacySingleProviderUnmarshal(t *testing.T) {
	raw := map[string]any{
		"issuer_url":     "https://issuer.example.com",
		"audience":       "test-audience",
		"username_claim": "email",
	}
	conf := confmap.NewFromStringMap(raw)
	config := &Config{}
	require.NoError(t, config.Unmarshal(conf))
	require.Len(t, config.Providers, 1)
	assert.Equal(t, "https://issuer.example.com", config.Providers[0].IssuerURL)
	assert.Equal(t, "test-audience", config.Providers[0].Audience)
	assert.Equal(t, "email", config.Providers[0].UsernameClaim)
	assert.True(t, config.hasLegacySingleProvider())
}

func TestLegacyAndMultipleProvidersMapping(t *testing.T) {
	config := &Config{
		IssuerURL: "https://legacy.example.com",
		Audience:  "legacy-audience",
		Providers: []ProviderCfg{
			{
				IssuerURL: "https://modern.example.com",
				Audience:  "modern-audience",
			},
		},
	}

	require.NoError(t, config.Validate())
	require.Len(t, config.Providers, 2)
	assert.Equal(t, "https://legacy.example.com", config.Providers[0].IssuerURL)
	assert.Equal(t, "https://modern.example.com", config.Providers[1].IssuerURL)
}

func TestLegacySingleProviderDeprecationWarning(t *testing.T) {
	core, logs := observer.New(zap.WarnLevel)
	logger := zap.New(core)

	config := &Config{
		IssuerURL: "https://example.com",
		Audience:  "aud",
	}

	ext := newExtension(config, logger)
	require.NotNil(t, ext)

	warns := logs.FilterMessageSnippet("The single-provider OIDC configuration is deprecated and will be removed in a future release")
	assert.Equal(t, 1, warns.Len())
}

func TestModernProvidersNoDeprecationWarning(t *testing.T) {
	core, logs := observer.New(zap.WarnLevel)
	logger := zap.New(core)

	config := &Config{
		Providers: []ProviderCfg{
			{
				IssuerURL: "https://example.com",
				Audience:  "aud",
			},
		},
	}

	ext := newExtension(config, logger)
	require.NotNil(t, ext)

	warns := logs.FilterMessageSnippet("The single-provider OIDC configuration is deprecated")
	assert.Equal(t, 0, warns.Len())
}
