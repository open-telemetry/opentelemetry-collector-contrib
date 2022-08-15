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

package attributesprocessor

import (
	"path/filepath"
	"testing"

	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/attraction"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/processor/filterconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/processor/filterset"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		id       config.ComponentID
		expected config.Processor
	}{
		{
			id: config.NewComponentIDWithName(typeStr, "insert"),
			expected: &Config{
				ProcessorSettings: config.NewProcessorSettings(config.NewComponentID(typeStr)),
				Settings: attraction.Settings{
					Actions: []attraction.ActionKeyValue{
						{Key: "attribute1", Value: 123, Action: attraction.INSERT},
						{Key: "string key", FromAttribute: "anotherkey", Action: attraction.INSERT},
					},
				},
			},
		},
		{
			id: config.NewComponentIDWithName(typeStr, "update"),
			expected: &Config{
				ProcessorSettings: config.NewProcessorSettings(config.NewComponentID(typeStr)),
				Settings: attraction.Settings{
					Actions: []attraction.ActionKeyValue{
						{Key: "boo", FromAttribute: "foo", Action: attraction.UPDATE},
						{Key: "db.secret", Value: "redacted", Action: attraction.UPDATE},
					},
				},
			},
		},
		{
			id: config.NewComponentIDWithName(typeStr, "upsert"),
			expected: &Config{
				ProcessorSettings: config.NewProcessorSettings(config.NewComponentID(typeStr)),
				Settings: attraction.Settings{
					Actions: []attraction.ActionKeyValue{
						{Key: "region", Value: "planet-earth", Action: attraction.UPSERT},
						{Key: "new_user_key", FromAttribute: "user_key", Action: attraction.UPSERT},
					},
				},
			},
		},
		{
			id: config.NewComponentIDWithName(typeStr, "delete"),
			expected: &Config{
				ProcessorSettings: config.NewProcessorSettings(config.NewComponentID(typeStr)),
				Settings: attraction.Settings{
					Actions: []attraction.ActionKeyValue{
						{Key: "credit_card", Action: attraction.DELETE},
						{Key: "duplicate_key", Action: attraction.DELETE},
					},
				},
			},
		},
		{
			id: config.NewComponentIDWithName(typeStr, "hash"),
			expected: &Config{
				ProcessorSettings: config.NewProcessorSettings(config.NewComponentID(typeStr)),
				Settings: attraction.Settings{
					Actions: []attraction.ActionKeyValue{
						{Key: "user.email", Action: attraction.HASH},
					},
				},
			},
		},
		{
			id: config.NewComponentIDWithName(typeStr, "excludemulti"),
			expected: &Config{
				ProcessorSettings: config.NewProcessorSettings(config.NewComponentID(typeStr)),
				MatchConfig: filterconfig.MatchConfig{
					Exclude: &filterconfig.MatchProperties{
						Config:   *createConfig(filterset.Strict),
						Services: []string{"svcA", "svcB"},
						Attributes: []filterconfig.Attribute{
							{Key: "env", Value: "dev"},
							{Key: "test_request"},
						},
					},
				},
				Settings: attraction.Settings{
					Actions: []attraction.ActionKeyValue{
						{Key: "credit_card", Action: attraction.DELETE},
						{Key: "duplicate_key", Action: attraction.DELETE},
					},
				},
			},
		},
		{
			id: config.NewComponentIDWithName(typeStr, "includeservices"),
			expected: &Config{
				ProcessorSettings: config.NewProcessorSettings(config.NewComponentID(typeStr)),
				MatchConfig: filterconfig.MatchConfig{
					Include: &filterconfig.MatchProperties{
						Config:   *createConfig(filterset.Regexp),
						Services: []string{"auth.*", "login.*"},
					},
				},
				Settings: attraction.Settings{
					Actions: []attraction.ActionKeyValue{
						{Key: "credit_card", Action: attraction.DELETE},
						{Key: "duplicate_key", Action: attraction.DELETE},
					},
				},
			},
		},
		{
			id: config.NewComponentIDWithName(typeStr, "selectiveprocessing"),
			expected: &Config{
				ProcessorSettings: config.NewProcessorSettings(config.NewComponentID(typeStr)),
				MatchConfig: filterconfig.MatchConfig{
					Include: &filterconfig.MatchProperties{
						Config:   *createConfig(filterset.Strict),
						Services: []string{"svcA", "svcB"},
					},
					Exclude: &filterconfig.MatchProperties{
						Config: *createConfig(filterset.Strict),
						Attributes: []filterconfig.Attribute{
							{Key: "redact_trace", Value: false},
						},
					},
				},
				Settings: attraction.Settings{
					Actions: []attraction.ActionKeyValue{
						{Key: "credit_card", Action: attraction.DELETE},
						{Key: "duplicate_key", Action: attraction.DELETE},
					},
				},
			},
		},
		{
			id: config.NewComponentIDWithName(typeStr, "complex"),
			expected: &Config{
				ProcessorSettings: config.NewProcessorSettings(config.NewComponentID(typeStr)),
				Settings: attraction.Settings{
					Actions: []attraction.ActionKeyValue{
						{Key: "operation", Value: "default", Action: attraction.INSERT},
						{Key: "svc.operation", FromAttribute: "operation", Action: attraction.UPSERT},
						{Key: "operation", Action: attraction.DELETE},
					},
				},
			},
		},
		{
			id: config.NewComponentIDWithName(typeStr, "example"),
			expected: &Config{
				ProcessorSettings: config.NewProcessorSettings(config.NewComponentID(typeStr)),
				Settings: attraction.Settings{
					Actions: []attraction.ActionKeyValue{
						{Key: "db.table", Action: attraction.DELETE},
						{Key: "redacted_span", Value: true, Action: attraction.UPSERT},
						{Key: "copy_key", FromAttribute: "key_original", Action: attraction.UPDATE},
						{Key: "account_id", Value: 2245, Action: attraction.INSERT},
						{Key: "account_password", Action: attraction.DELETE},
					},
				},
			},
		},
		{
			id: config.NewComponentIDWithName(typeStr, "regexp"),
			expected: &Config{
				ProcessorSettings: config.NewProcessorSettings(config.NewComponentID(typeStr)),
				MatchConfig: filterconfig.MatchConfig{
					Include: &filterconfig.MatchProperties{
						Config:   *createConfig(filterset.Regexp),
						Services: []string{"auth.*"},
					},
					Exclude: &filterconfig.MatchProperties{
						Config:    *createConfig(filterset.Regexp),
						SpanNames: []string{"login.*"},
					},
				},
				Settings: attraction.Settings{
					Actions: []attraction.ActionKeyValue{
						{Key: "password", Action: attraction.UPDATE, Value: "obfuscated"},
						{Key: "token", Action: attraction.DELETE},
					},
				},
			},
		},
		{
			id: config.NewComponentIDWithName(typeStr, "convert"),
			expected: &Config{
				ProcessorSettings: config.NewProcessorSettings(config.NewComponentID(typeStr)),
				Settings: attraction.Settings{
					Actions: []attraction.ActionKeyValue{
						{Key: "http.status_code", Action: attraction.CONVERT, ConvertedType: "int"},
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
			require.NoError(t, config.UnmarshalProcessor(sub, cfg))

			assert.NoError(t, cfg.Validate())
			assert.Equal(t, tt.expected, cfg)
		})
	}
}
