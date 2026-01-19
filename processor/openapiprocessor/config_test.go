// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package openapiprocessor

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
)

func TestLoadingConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		id       component.ID
		expected component.Config
	}{
		{
			id: component.MustNewIDWithName("openapi", "basic"),
			expected: &Config{
				OpenAPIFile:          "testdata/openapi.yaml",
				URLAttribute:         "http.url",
				URLTemplateAttribute: "url.template",
				PeerServiceAttribute: "peer.service",
				OverwriteExisting:    false,
				IncludeQueryParams:   false,
				UseServerURLMatching: false,
			},
		},
		{
			id: component.MustNewIDWithName("openapi", "custom"),
			expected: &Config{
				OpenAPIFile:          "testdata/openapi.json",
				URLAttribute:         "url.path",
				URLTemplateAttribute: "http.route",
				PeerServiceAttribute: "service.name",
				PeerService:          "my-api-service",
				OverwriteExisting:    true,
				IncludeQueryParams:   true,
				UseServerURLMatching: false,
			},
		},
		{
			id: component.MustNewIDWithName("openapi", "template_peer_service"),
			expected: &Config{
				OpenAPIFile:          "testdata/openapi.yaml",
				URLAttribute:         "http.url",
				URLTemplateAttribute: "url.template",
				PeerServiceAttribute: "peer.service",
				PeerService:          "${info.title}-backend",
				OverwriteExisting:    false,
				IncludeQueryParams:   false,
				UseServerURLMatching: false,
			},
		},
		{
			id: component.MustNewIDWithName("openapi", "server_matching"),
			expected: &Config{
				OpenAPIFile:          "testdata/openapi.yaml",
				URLAttribute:         "http.url",
				URLTemplateAttribute: "url.template",
				PeerServiceAttribute: "peer.service",
				OverwriteExisting:    false,
				IncludeQueryParams:   false,
				UseServerURLMatching: true,
			},
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
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: &Config{
				OpenAPIFile: "testdata/openapi.yaml",
			},
			wantErr: false,
		},
		{
			name: "missing openapi_file",
			config: &Config{
				OpenAPIFile: "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
