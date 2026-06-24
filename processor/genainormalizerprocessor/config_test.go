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
			name: "valid openinference source",
			cfg: Config{
				Sources: []Source{{Name: SourceOpenInference, RemoveOriginals: true}},
			},
		},
		{
			name: "valid openllmetry source",
			cfg: Config{
				Sources: []Source{{Name: SourceOpenLLMetry, RemoveOriginals: true}},
			},
		},
		{
			name: "both sources",
			cfg: Config{
				Sources: []Source{
					{Name: SourceOpenInference},
					{Name: SourceOpenLLMetry},
				},
			},
		},
		{
			name: "all built-in sources",
			cfg: Config{
				Sources: []Source{
					{Name: SourceOpenInference},
					{Name: SourceOpenLLMetry},
					{Name: SourceLangChain},
					{Name: SourceCrewAI},
					{Name: SourcePydanticAI},
				},
			},
		},
		{
			name: "built-in source with mappings set (crewai)",
			cfg: Config{
				Sources: []Source{{
					Name:     SourceCrewAI,
					Mappings: map[string]string{"foo": "gen_ai.agent.name"},
				}},
			},
			wantErr: `sources[0]: "mappings" is not valid on built-in source "crewai"`,
		},
		{
			name:    "no sources",
			cfg:     Config{Sources: []Source{}},
			wantErr: "at least one source",
		},
		{
			name: "empty name",
			cfg: Config{
				Sources: []Source{{Name: ""}},
			},
			wantErr: `sources[0]: "name" must be set`,
		},
		{
			name: "duplicate built-in source",
			cfg: Config{
				Sources: []Source{
					{Name: SourceOpenInference},
					{Name: SourceOpenInference},
				},
			},
			wantErr: `sources[1]: duplicate source "openinference"`,
		},
		{
			name: "duplicate user-defined source",
			cfg: Config{
				Sources: []Source{
					{Name: "my_vendor", Mappings: map[string]string{"a": "gen_ai.request.model"}},
					{Name: "my_vendor", Mappings: map[string]string{"b": "gen_ai.request.model"}},
				},
			},
			wantErr: `sources[1]: duplicate source "my_vendor"`,
		},
		{
			name: "valid user-defined source",
			cfg: Config{
				Sources: []Source{{
					Name:     "my_vendor",
					Mappings: map[string]string{"my_vendor.model": "gen_ai.request.model"},
				}},
			},
		},
		{
			name: "user-defined source with empty mappings",
			cfg: Config{
				Sources: []Source{{Name: "my_vendor"}},
			},
			wantErr: `sources[0]: user-defined source "my_vendor" requires non-empty "mappings" (built-in sources, which do not require mappings: crewai, langchain, openinference, openllmetry, pydanticai)`,
		},
		{
			name: "typo on built-in source name surfaces built-in list",
			cfg: Config{
				Sources: []Source{{Name: "openllllmetry"}},
			},
			wantErr: `built-in sources, which do not require mappings: crewai, langchain, openinference, openllmetry, pydanticai`,
		},
		{
			name: "built-in source with mappings set",
			cfg: Config{
				Sources: []Source{{
					Name:     SourceOpenInference,
					Mappings: map[string]string{"foo": "gen_ai.request.model"},
				}},
			},
			wantErr: `sources[0]: "mappings" is not valid on built-in source "openinference"`,
		},
		{
			name: "two distinct user-defined sources allowed",
			cfg: Config{
				Sources: []Source{
					{Name: "vendor_a", Mappings: map[string]string{"vendor_a.model": "gen_ai.request.model"}},
					{Name: "vendor_b", Mappings: map[string]string{"vendor_b.model": "gen_ai.request.model"}},
				},
			},
		},
		{
			name: "valid user-defined source with value_mappings",
			cfg: Config{
				Sources: []Source{{
					Name:     "my_vendor",
					Mappings: map[string]string{"my_vendor.op": "gen_ai.operation.name"},
					ValueMappings: map[string]map[string]string{
						"gen_ai.operation.name": {"chat_completion": "chat"},
					},
				}},
			},
		},
		{
			name: "built-in source with value_mappings set",
			cfg: Config{
				Sources: []Source{{
					Name: SourceOpenInference,
					ValueMappings: map[string]map[string]string{
						"gen_ai.operation.name": {"chat_completion": "chat"},
					},
				}},
			},
			wantErr: `sources[0]: "value_mappings" is not valid on built-in source "openinference"`,
		},
		{
			name: "value_mappings key not in mappings targets",
			cfg: Config{
				Sources: []Source{{
					Name:     "my_vendor",
					Mappings: map[string]string{"my_vendor.model": "gen_ai.request.model"},
					ValueMappings: map[string]map[string]string{
						"gen_ai.operation.name": {"chat_completion": "chat"},
					},
				}},
			},
			wantErr: `sources[0]: "value_mappings" key "gen_ai.operation.name" is not a target in "mappings"`,
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
				Sources: []Source{
					{
						Name:            SourceOpenInference,
						RemoveOriginals: true,
						Overwrite:       false,
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
				Sources: []Source{{Name: SourceOpenInference}},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "openllmetry_only"),
			expected: &Config{
				Sources: []Source{{Name: SourceOpenLLMetry}},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "openinference_and_openllmetry"),
			expected: &Config{
				Sources: []Source{
					{Name: SourceOpenInference, RemoveOriginals: true},
					{Name: SourceOpenLLMetry, RemoveOriginals: true},
				},
			},
		},
		{
			id:           component.NewIDWithName(metadata.Type, "empty_sources"),
			errorMessage: "at least one source must be specified",
		},
		{
			id:           component.NewIDWithName(metadata.Type, "duplicate_source"),
			errorMessage: `duplicate source "openinference"`,
		},
		{
			id: component.NewIDWithName(metadata.Type, "user_defined_only"),
			expected: &Config{
				Sources: []Source{{
					Name:            "my_vendor",
					RemoveOriginals: true,
					Mappings: map[string]string{
						"my_vendor.model":     "gen_ai.request.model",
						"my_vendor.tokens.in": "gen_ai.usage.input_tokens",
					},
				}},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "user_defined_with_builtin"),
			expected: &Config{
				Sources: []Source{
					{Name: SourceOpenInference, RemoveOriginals: true},
					{
						Name:            "my_vendor",
						RemoveOriginals: true,
						Mappings:        map[string]string{"my_vendor.model": "gen_ai.request.model"},
					},
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "multiple_user_defined"),
			expected: &Config{
				Sources: []Source{
					{
						Name:            "vendor_a",
						RemoveOriginals: true,
						Mappings:        map[string]string{"vendor_a.model": "gen_ai.request.model"},
					},
					{
						Name:            "vendor_b",
						RemoveOriginals: true,
						Overwrite:       true,
						Mappings:        map[string]string{"vendor_b.model": "gen_ai.request.model"},
					},
				},
			},
		},
		{
			id:           component.NewIDWithName(metadata.Type, "user_defined_empty_mappings"),
			errorMessage: `user-defined source "my_vendor" requires non-empty "mappings" (built-in sources, which do not require mappings: crewai, langchain, openinference, openllmetry, pydanticai)`,
		},
		{
			id:           component.NewIDWithName(metadata.Type, "openinference_with_mappings"),
			errorMessage: `"mappings" is not valid on built-in source "openinference"`,
		},
		{
			id: component.NewIDWithName(metadata.Type, "new_builtin_sources"),
			expected: &Config{
				Sources: []Source{
					{Name: SourceLangChain, RemoveOriginals: true},
					{Name: SourceCrewAI},
					{Name: SourcePydanticAI},
				},
			},
		},
		{
			id:           component.NewIDWithName(metadata.Type, "user_defined_unreachable_value_mapping"),
			errorMessage: `"value_mappings" key "gen_ai.operation.name" is not a target in "mappings"`,
		},
		{
			id: component.NewIDWithName(metadata.Type, "user_defined_with_value_mappings"),
			expected: &Config{
				Sources: []Source{{
					Name:            "my_vendor",
					RemoveOriginals: true,
					Mappings:        map[string]string{"my_vendor.op": "gen_ai.operation.name"},
					ValueMappings: map[string]map[string]string{
						"gen_ai.operation.name": {
							"chat_completion": "chat",
							"tool_invoke":     "execute_tool",
						},
					},
				}},
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

			if tt.errorMessage != "" {
				assert.ErrorContains(t, xconfmap.Validate(cfg), tt.errorMessage)
				return
			}
			require.NoError(t, xconfmap.Validate(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}
