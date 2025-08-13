// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oauth2clientauthextension

import (
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/oauth2clientauthextension/internal/metadata"
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
				ClientSecret:   "someclientsecret",
				ClientID:       "someclientid",
				EndpointParams: url.Values{"audience": []string{"someaudience"}},
				Scopes:         []string{"api.metrics"},
				TokenURL:       "https://example.com/oauth2/default/v1/token",
				Timeout:        time.Second,
				ExpiryBuffer:   5 * time.Minute,
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "withtls"),
			expected: &Config{
				ClientSecret: "someclientsecret2",
				ClientID:     "someclientid2",
				Scopes:       []string{"api.metrics"},
				TokenURL:     "https://example2.com/oauth2/default/v1/token",
				Timeout:      time.Second,
				TLS: configtls.ClientConfig{
					Config: configtls.Config{
						CAFile:   "cafile",
						CertFile: "certfile",
						KeyFile:  "keyfile",
					},
					Insecure:           true,
					InsecureSkipVerify: false,
					ServerName:         "",
				},
				ExpiryBuffer: 15 * time.Second,
			},
		},
		{
			id:          component.NewIDWithName(metadata.Type, "missingurl"),
			expectedErr: errNoTokenURLProvided,
		},
		{
			id:          component.NewIDWithName(metadata.Type, "missingid"),
			expectedErr: errNoClientIDProvided,
		},
		{
			id:          component.NewIDWithName(metadata.Type, "missingsecret"),
			expectedErr: errNoClientSecretProvided,
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
