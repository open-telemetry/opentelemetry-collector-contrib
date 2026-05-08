// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package genainormalizerprocessor

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/genainormalizerprocessor/internal/metadata"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr string
	}{
		{
			name: "valid built-in sources",
			cfg: Config{
				Sources: map[SourceName]Source{
					SourceOpenInference: {RemoveOriginals: true},
					SourceOpenLLMetry:   {},
				},
			},
		},
		{
			name: "valid custom source",
			cfg: Config{
				Sources: map[SourceName]Source{
					SourceCustom: {
						CustomMappings: map[string]string{"my_vendor.model": "gen_ai.request.model"},
					},
				},
			},
		},
		{
			name: "valid mix of built-in and custom",
			cfg: Config{
				Sources: map[SourceName]Source{
					SourceOpenInference: {},
					SourceCustom: {
						CustomMappings: map[string]string{"my_vendor.model": "gen_ai.request.model"},
					},
				},
			},
		},
		{
			name:    "no sources",
			cfg:     Config{Sources: map[SourceName]Source{}},
			wantErr: "at least one source",
		},
		{
			name: "unknown source",
			cfg: Config{
				Sources: map[SourceName]Source{"bogus": {}},
			},
			wantErr: `unknown source "bogus"`,
		},
		{
			name: "custom source without mappings",
			cfg: Config{
				Sources: map[SourceName]Source{
					SourceCustom: {},
				},
			},
			wantErr: "custom_mappings must be non-empty for the custom source",
		},
		{
			name: "custom_mappings empty source attr",
			cfg: Config{
				Sources: map[SourceName]Source{
					SourceOpenInference: {
						CustomMappings: map[string]string{"": "gen_ai.request.model"},
					},
				},
			},
			wantErr: "source attribute name must be non-empty",
		},
		{
			name: "custom_mappings empty target",
			cfg: Config{
				Sources: map[SourceName]Source{
					SourceOpenInference: {
						CustomMappings: map[string]string{"my_vendor.model": ""},
					},
				},
			},
			wantErr: `target for "my_vendor.model" must be non-empty`,
		},
		{
			name: "custom_mappings identity",
			cfg: Config{
				Sources: map[SourceName]Source{
					SourceOpenInference: {
						CustomMappings: map[string]string{"gen_ai.request.model": "gen_ai.request.model"},
					},
				},
			},
			wantErr: "source and target are identical",
		},
		{
			name: "valid custom_mappings on built-in source",
			cfg: Config{
				Sources: map[SourceName]Source{
					SourceOpenInference: {
						CustomMappings: map[string]string{"my_vendor.model": "gen_ai.request.model"},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tt.wantErr)
			}
		})
	}
}

// TestDefaultConfigIsInvalid ensures the factory-returned default config fails
// validation, since the user must explicitly configure at least one source.
func TestDefaultConfigIsInvalid(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	assert.Empty(t, cfg.Sources)
	require.ErrorContains(t, cfg.Validate(), "at least one source must be specified")
}

// TestLoadConfig loads each named config from testdata/config.yaml, unmarshals
// it into a Config and either asserts the expected shape (for valid configs)
// or asserts the expected validation error message (for invalid configs).
func TestLoadConfig(t *testing.T) {
	tests := []struct {
		id           component.ID
		expected     *Config
		errorMessage string
	}{
		{
			id: component.NewIDWithName(metadata.Type, ""),
			expected: &Config{
				Sources: map[SourceName]Source{
					SourceOpenInference: {
						RemoveOriginals: true,
						Overwrite:       false,
						CustomMappings:  map[string]string{"my_vendor.model": "gen_ai.request.model"},
					},
					SourceOpenLLMetry: {RemoveOriginals: true},
					SourceCustom: {
						CustomMappings: map[string]string{"other_vendor.tokens": "gen_ai.usage.input_tokens"},
					},
				},
			},
		},
		{
			id:           component.NewIDWithName(metadata.Type, "empty"),
			errorMessage: "at least one source must be specified",
		},
		{
			id: component.NewIDWithName(metadata.Type, "openinference_only"),
			expected: &Config{
				Sources: map[SourceName]Source{
					SourceOpenInference: {},
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "openllmetry_only"),
			expected: &Config{
				Sources: map[SourceName]Source{
					SourceOpenLLMetry: {RemoveOriginals: true},
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "custom_only"),
			expected: &Config{
				Sources: map[SourceName]Source{
					SourceCustom: {
						CustomMappings: map[string]string{"my_vendor.model": "gen_ai.request.model"},
					},
				},
			},
		},
		{
			id:           component.NewIDWithName(metadata.Type, "empty_sources"),
			errorMessage: "at least one source must be specified",
		},
		{
			id:           component.NewIDWithName(metadata.Type, "custom_missing_mappings"),
			errorMessage: "custom_mappings must be non-empty for the custom source",
		},
		{
			id:           component.NewIDWithName(metadata.Type, "unknown_source"),
			errorMessage: `unknown source "foobar"`,
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

			if tt.errorMessage != "" {
				assert.ErrorContains(t, xconfmap.Validate(cfg), tt.errorMessage)
				return
			}
			require.NoError(t, xconfmap.Validate(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}
