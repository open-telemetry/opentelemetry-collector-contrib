// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package headerssetterextension

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/headerssetterextension/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		id            component.ID
		expected      component.Config
		expectedError error
	}{
		{
			id:            component.NewIDWithName(metadata.Type, ""),
			expectedError: errMissingHeadersConfig,
		},
		{
			id: component.NewIDWithName(metadata.Type, "1"),
			expected: &Config{
				HeadersConfig: []HeaderConfig{
					{
						Key:         stringp("X-Scope-OrgID"),
						Action:      INSERT,
						FromContext: stringp("tenant_id"),
						Value:       nil,
					},
					{
						Key:         stringp("User-ID"),
						Action:      UPDATE,
						FromContext: stringp("user_id"),
						Value:       nil,
					},

					{
						Key:         stringp("User-ID"),
						FromContext: nil,
						Value:       stringp("user_id"),
					},
					{
						Key:    stringp("User-ID"),
						Action: DELETE,
					},
				},
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
			require.NoError(t, component.UnmarshalConfig(sub, cfg))

			if tt.expectedError != nil {
				assert.Error(t, component.ValidateConfig(cfg), tt.expectedError)
				return
			}
			assert.NoError(t, component.ValidateConfig(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name        string
		header      []HeaderConfig
		expectedErr error
	}{
		{
			"header value from config property",
			[]HeaderConfig{
				{
					Key:    stringp("name"),
					Action: INSERT,
					Value:  stringp("from config"),
				},
			},
			nil,
		},
		{
			"header value from context",
			[]HeaderConfig{
				{
					Key:         stringp("name"),
					Action:      INSERT,
					FromContext: stringp("from config"),
				},
			},
			nil,
		},
		{
			"missing header name for from value",
			[]HeaderConfig{
				{
					Action: INSERT,
					Value:  stringp("test"),
				},
			},
			errMissingHeader,
		},
		{
			"missing header name for from context",
			[]HeaderConfig{
				{
					Action:      INSERT,
					FromContext: stringp("test"),
				},
			},
			errMissingHeader,
		},
		{
			"header value from context and value",
			[]HeaderConfig{
				{
					Key:         stringp("name"),
					Action:      INSERT,
					Value:       stringp("from config"),
					FromContext: stringp("from context"),
				},
			},
			errConflictingSources,
		},
		{
			"header value source is missing",
			[]HeaderConfig{
				{
					Key:    stringp("name"),
					Action: INSERT,
				},
			},
			errMissingSource,
		},
		{
			"delete header action",
			[]HeaderConfig{
				{
					Key:    stringp("name"),
					Action: DELETE,
				},
			},
			nil,
		},
		{
			"insert header action",
			[]HeaderConfig{
				{
					Key:    stringp("name"),
					Action: INSERT,
					Value:  stringp("from config"),
				},
			},
			nil,
		},
		{
			"missing header action",
			[]HeaderConfig{
				{
					Key:   stringp("name"),
					Value: stringp("from config"),
				},
			},
			nil,
		},
		{
			"headers configuration is missing",
			nil,
			errMissingHeadersConfig,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{HeadersConfig: tt.header}
			require.ErrorIs(t, cfg.Validate(), tt.expectedErr)
		})
	}
}
