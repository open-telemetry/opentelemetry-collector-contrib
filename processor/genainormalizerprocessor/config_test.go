// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package genainormalizerprocessor

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
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

// TestUnmarshal covers the WYSIWYG behavior of the `sources` map: defaults
// apply only when the user omits `sources` entirely; any user-specified value
// replaces the defaults.
func TestUnmarshal(t *testing.T) {
	tests := []struct {
		name        string
		userConf    map[string]any
		wantSources map[SourceName]Source
	}{
		{
			name:     "empty config keeps defaults",
			userConf: map[string]any{},
			wantSources: map[SourceName]Source{
				SourceOpenInference: {},
				SourceOpenLLMetry:   {},
			},
		},
		{
			name: "only openinference replaces defaults",
			userConf: map[string]any{
				"sources": map[string]any{
					"openinference": map[string]any{
						"remove_originals": true,
					},
				},
			},
			wantSources: map[SourceName]Source{
				SourceOpenInference: {RemoveOriginals: true},
			},
		},
		{
			name: "only openllmetry replaces defaults",
			userConf: map[string]any{
				"sources": map[string]any{
					"openllmetry": map[string]any{},
				},
			},
			wantSources: map[SourceName]Source{
				SourceOpenLLMetry: {},
			},
		},
		{
			name: "only custom replaces defaults",
			userConf: map[string]any{
				"sources": map[string]any{
					"custom": map[string]any{
						"custom_mappings": map[string]string{
							"my_vendor.model": "gen_ai.request.model",
						},
					},
				},
			},
			wantSources: map[SourceName]Source{
				SourceCustom: {
					CustomMappings: map[string]string{"my_vendor.model": "gen_ai.request.model"},
				},
			},
		},
		{
			name: "built-in and custom together",
			userConf: map[string]any{
				"sources": map[string]any{
					"openinference": map[string]any{},
					"custom": map[string]any{
						"custom_mappings": map[string]string{
							"my_vendor.model": "gen_ai.request.model",
						},
					},
				},
			},
			wantSources: map[SourceName]Source{
				SourceOpenInference: {},
				SourceCustom: {
					CustomMappings: map[string]string{"my_vendor.model": "gen_ai.request.model"},
				},
			},
		},
		{
			name: "explicit empty sources wipes defaults",
			userConf: map[string]any{
				"sources": map[string]any{},
			},
			wantSources: map[SourceName]Source{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)
			conf := confmap.NewFromStringMap(tt.userConf)
			require.NoError(t, cfg.Unmarshal(conf))
			assert.Equal(t, tt.wantSources, cfg.Sources)
		})
	}
}

// TestDefaultConfigIsValid ensures that the factory-returned default config
// passes Validate (callers that never set `sources` should not get an error).
func TestDefaultConfigIsValid(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	require.NoError(t, cfg.Validate())
	assert.Contains(t, cfg.Sources, SourceOpenInference)
	assert.Contains(t, cfg.Sources, SourceOpenLLMetry)
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
			id:       component.NewIDWithName(metadata.Type, "defaults"),
			expected: createDefaultConfig().(*Config),
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
