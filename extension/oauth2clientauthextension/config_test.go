// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package oauth2clientauthextension

import (
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap/confmaptest"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		id          config.ComponentID
		expected    config.Extension
		expectedErr error
	}{
		{
			id: config.NewComponentID(typeStr),
			expected: &Config{
				ExtensionSettings: config.NewExtensionSettings(config.NewComponentID(typeStr)),
				ClientSecret:      "someclientsecret",
				ClientID:          "someclientid",
				EndpointParams:    url.Values{"audience": []string{"someaudience"}},
				Scopes:            []string{"api.metrics"},
				TokenURL:          "https://example.com/oauth2/default/v1/token",
				Timeout:           time.Second,
			},
		},
		{
			id: config.NewComponentIDWithName(typeStr, "withtls"),
			expected: &Config{
				ExtensionSettings: config.NewExtensionSettings(config.NewComponentID(typeStr)),
				ClientSecret:      "someclientsecret2",
				ClientID:          "someclientid2",
				Scopes:            []string{"api.metrics"},
				TokenURL:          "https://example2.com/oauth2/default/v1/token",
				Timeout:           time.Second,
				TLSSetting: configtls.TLSClientSetting{
					TLSSetting: configtls.TLSSetting{
						CAFile:   "cafile",
						CertFile: "certfile",
						KeyFile:  "keyfile",
					},
					Insecure:           true,
					InsecureSkipVerify: false,
					ServerName:         "",
				},
			},
		},
		{
			id:          config.NewComponentIDWithName(typeStr, "missingurl"),
			expectedErr: errNoTokenURLProvided,
		},
		{
			id:          config.NewComponentIDWithName(typeStr, "missingid"),
			expectedErr: errNoClientIDProvided,
		},
		{
			id:          config.NewComponentIDWithName(typeStr, "missingsecret"),
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
			require.NoError(t, config.UnmarshalExtension(sub, cfg))
			if tt.expectedErr != nil {
				assert.ErrorIs(t, cfg.Validate(), tt.expectedErr)
				return
			}
			assert.NoError(t, cfg.Validate())
			assert.Equal(t, tt.expected, cfg)
		})
	}
}
