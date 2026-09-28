// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package openshift

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configtls"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/openshift/internal/metadata"
)

func TestMergeWithDefaults(t *testing.T) {
	tt := []struct {
		name     string
		cfg      Config
		host     string
		port     string
		expected Config
		wantErr  string
	}{
		{
			name:     "explicit values kept and default CA applied",
			cfg:      Config{Address: "https://api:6443", Token: "token"},
			expected: Config{Address: "https://api:6443", Token: "token", TLSs: configtls.ClientConfig{Config: configtls.Config{CAFile: defaultCAPath}}},
		},
		{
			name:     "insecure skips default CA",
			cfg:      Config{Address: "https://api:6443", Token: "token", TLSs: configtls.ClientConfig{Insecure: true}},
			expected: Config{Address: "https://api:6443", Token: "token", TLSs: configtls.ClientConfig{Insecure: true}},
		},
		{
			name:     "explicit CA file kept",
			cfg:      Config{Address: "https://api:6443", Token: "token", TLSs: configtls.ClientConfig{Config: configtls.Config{CAFile: "/my/ca.crt"}}},
			expected: Config{Address: "https://api:6443", Token: "token", TLSs: configtls.ClientConfig{Config: configtls.Config{CAFile: "/my/ca.crt"}}},
		},
		{
			name:     "address from environment",
			cfg:      Config{Token: "token", TLSs: configtls.ClientConfig{Insecure: true}},
			host:     "10.0.0.1",
			port:     "443",
			expected: Config{Address: "https://10.0.0.1:443", Token: "token", TLSs: configtls.ClientConfig{Insecure: true}},
		},
		{
			name:    "missing service host",
			cfg:     Config{Token: "token"},
			port:    "443",
			wantErr: "could not extract openshift api host",
		},
		{
			name:    "missing service port",
			cfg:     Config{Token: "token"},
			host:    "10.0.0.1",
			wantErr: "could not extract openshift api port",
		},
		{
			name:    "missing token and no in-cluster token file",
			cfg:     Config{Address: "https://api:6443"},
			wantErr: defaultServiceTokenPath,
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("KUBERNETES_SERVICE_HOST", tc.host)
			t.Setenv("KUBERNETES_SERVICE_PORT", tc.port)

			err := tc.cfg.MergeWithDefaults()
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.expected, tc.cfg)
		})
	}
}

func TestCreateDefaultConfig(t *testing.T) {
	assert.Equal(t, Config{ResourceAttributes: metadata.DefaultResourceAttributesConfig()}, CreateDefaultConfig())
}
