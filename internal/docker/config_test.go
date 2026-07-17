// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package docker // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/docker"

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap"
)

func TestAPIVersion(t *testing.T) {
	for _, test := range []struct {
		input           string
		expectedVersion string
		expectedRep     string
		expectedError   string
	}{
		{
			input:           "1.2",
			expectedVersion: MustNewAPIVersion("1.2"),
			expectedRep:     "1.2",
		},
		{
			input:           "1.40",
			expectedVersion: MustNewAPIVersion("1.40"),
			expectedRep:     "1.40",
		},
		{
			input:           "10",
			expectedVersion: MustNewAPIVersion("10.0"),
			expectedRep:     "10.0",
		},
		{
			input:           "0",
			expectedVersion: MustNewAPIVersion("0.0"),
			expectedRep:     "0.0",
		},
		{
			input:           ".400",
			expectedVersion: MustNewAPIVersion("0.400"),
			expectedRep:     "0.400",
		},
		{
			input:           "00000.400",
			expectedVersion: MustNewAPIVersion("0.400"),
			expectedRep:     "0.400",
		},
		{
			input:         "0.1.",
			expectedError: `invalid version "0.1."`,
		},
		{
			input:         "0.1.2.3",
			expectedError: `invalid version "0.1.2.3"`,
		},
		{
			input:         "",
			expectedError: `invalid version ""`,
		},
		{
			input:         "...",
			expectedError: `invalid version "..."`,
		},
	} {
		t.Run(test.input, func(t *testing.T) {
			version, err := NewAPIVersion(test.input)
			if test.expectedError != "" {
				assert.EqualError(t, err, test.expectedError)
				assert.Empty(t, version)
				return
			}
			require.NoError(t, err)
			require.Equal(t, test.expectedVersion, version)
			require.Equal(t, test.expectedRep, version)
			require.Equal(t, test.expectedRep, version)
		})
	}
}

func TestDefaultConfigHasNoTLS(t *testing.T) {
	cfg := NewDefaultConfig()
	assert.False(t, cfg.TLS.HasValue(), "default config should have nil TLS for backward compatibility")
}

func TestNewConfigHasNoTLS(t *testing.T) {
	cfg := NewConfig("unix:///var/run/docker.sock", 0, nil, "")
	assert.False(t, cfg.TLS.HasValue(), "NewConfig should have nil TLS by default")
}

func TestUnmarshalNoTLSKeepsNil(t *testing.T) {
	conf := confmap.NewFromStringMap(map[string]any{
		"endpoint": "unix:///var/run/docker.sock",
	})
	cfg := &Config{}
	require.NoError(t, cfg.Unmarshal(conf))
	assert.False(t, cfg.TLS.HasValue(), "omitting tls block should leave TLS nil")
}

func TestUnmarshalWithTLSBlock(t *testing.T) {
	conf := confmap.NewFromStringMap(map[string]any{
		"endpoint": "https://example.com/",
		"tls": map[string]any{
			"insecure_skip_verify": true,
		},
	})
	cfg := &Config{}
	require.NoError(t, cfg.Unmarshal(conf))
	assert.True(t, cfg.TLS.HasValue())
	assert.True(t, cfg.TLS.Get().InsecureSkipVerify)
}

func TestValidateInvalidTLSCertPath(t *testing.T) {
	cfg := &Config{
		Endpoint: "https://example.com/",
		TLS: configoptional.Some(configtls.ClientConfig{
			Config: configtls.Config{
				CAFile: "/nonexistent/ca.pem",
			},
		}),
	}
	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid tls configuration")
}
