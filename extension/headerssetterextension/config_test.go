// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package headerssetterextension

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/confmap"
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
						Key:         new("X-Scope-OrgID"),
						Action:      INSERT,
						FromContext: new("tenant_id"),
						Value:       nil,
					},
					{
						Key:          new("X-Scope-OrgID"),
						Action:       INSERT,
						FromContext:  new("tenant_id"),
						DefaultValue: new(configopaque.String("some_id")),
						Value:        nil,
					},
					{
						Key:         new("User-ID"),
						Action:      UPDATE,
						FromContext: new("user_id"),
						Value:       nil,
					},

					{
						Key:         new("User-ID"),
						FromContext: nil,
						Value:       new("user_id"),
					},
					{
						Key:    new("User-ID"),
						Action: DELETE,
					},
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "2"),
			expected: &Config{
				AdditionalAuth: func() *component.ID {
					id := component.MustNewID("oauth2client")
					return &id
				}(),
				HeadersConfig: []HeaderConfig{
					{
						Key:    new("X-Custom-Header"),
						Value:  new("custom-value"),
						Action: UPSERT,
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
			require.NoError(t, sub.Unmarshal(cfg))

			if tt.expectedError != nil {
				assert.ErrorIs(t, confmap.Validate(cfg), tt.expectedError)
				return
			}
			assert.NoError(t, confmap.Validate(cfg))
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
					Key:    new("name"),
					Action: INSERT,
					Value:  new("from config"),
				},
			},
			nil,
		},
		{
			"header value from context",
			[]HeaderConfig{
				{
					Key:         new("name"),
					Action:      INSERT,
					FromContext: new("from config"),
				},
			},
			nil,
		},
		{
			"missing header name for from value",
			[]HeaderConfig{
				{
					Action: INSERT,
					Value:  new("test"),
				},
			},
			errMissingHeader,
		},
		{
			"missing header name for from context",
			[]HeaderConfig{
				{
					Action:      INSERT,
					FromContext: new("test"),
				},
			},
			errMissingHeader,
		},
		{
			"header value from context and value",
			[]HeaderConfig{
				{
					Key:         new("name"),
					Action:      INSERT,
					Value:       new("from config"),
					FromContext: new("from context"),
				},
			},
			errConflictingSources,
		},
		{
			"header value source is missing",
			[]HeaderConfig{
				{
					Key:    new("name"),
					Action: INSERT,
				},
			},
			errMissingSource,
		},
		{
			"header value source is missing snd default value set",
			[]HeaderConfig{
				{
					Key:          new("name"),
					Action:       INSERT,
					FromContext:  new("from context"),
					DefaultValue: new(configopaque.String("default")),
				},
			},
			nil,
		},
		{
			"delete header action",
			[]HeaderConfig{
				{
					Key:    new("name"),
					Action: DELETE,
				},
			},
			nil,
		},
		{
			"insert header action",
			[]HeaderConfig{
				{
					Key:    new("name"),
					Action: INSERT,
					Value:  new("from config"),
				},
			},
			nil,
		},
		{
			"missing header action",
			[]HeaderConfig{
				{
					Key:   new("name"),
					Value: new("from config"),
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
