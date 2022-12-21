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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/attraction"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterset"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		id       component.ID
		expected component.Config
	}{
		{
			id: component.NewIDWithName(typeStr, "insert"),
			expected: &Config{
				Settings: attraction.Settings{
					Actions: []attraction.ActionKeyValue{
						{Key: "attribute1", Value: 123, Action: attraction.INSERT},
						{Key: "string key", FromAttribute: "anotherkey", Action: attraction.INSERT},
					},
				},
			},
		},
		{
			id: component.NewIDWithName(typeStr, "update"),
			expected: &Config{
				Settings: attraction.Settings{
					Actions: []attraction.ActionKeyValue{
						{Key: "boo", FromAttribute: "foo", Action: attraction.UPDATE},
						{Key: "db.secret", Value: "redacted", Action: attraction.UPDATE},
					},
				},
			},
		},
		{
			id: component.NewIDWithName(typeStr, "upsert"),
			expected: &Config{
				Settings: attraction.Settings{
					Actions: []attraction.ActionKeyValue{
						{Key: "region", Value: "planet-earth", Action: attraction.UPSERT},
						{Key: "new_user_key", FromAttribute: "user_key", Action: attraction.UPSERT},
					},
				},
			},
		},
		{
			id: component.NewIDWithName(typeStr, "delete"),
			expected: &Config{
				Settings: attraction.Settings{
					Actions: []attraction.ActionKeyValue{
						{Key: "credit_card", Action: attraction.DELETE},
						{Key: "duplicate_key", Action: attraction.DELETE},
					},
				},
			},
		},
		{
			id: component.NewIDWithName(typeStr, "hash"),
			expected: &Config{
				Settings: attraction.Settings{
					Actions: []attraction.ActionKeyValue{
						{Key: "user.email", Action: attraction.HASH},
					},
				},
			},
		},
		{
			id: component.NewIDWithName(typeStr, "excludemulti"),
			expected: &Config{
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
			id: component.NewIDWithName(typeStr, "includeservices"),
			expected: &Config{
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
			id: component.NewIDWithName(typeStr, "selectiveprocessing"),
			expected: &Config{
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
			id: component.NewIDWithName(typeStr, "complex"),
			expected: &Config{
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
			id: component.NewIDWithName(typeStr, "example"),
			expected: &Config{
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
			id: component.NewIDWithName(typeStr, "regexp"),
			expected: &Config{
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
			id: component.NewIDWithName(typeStr, "convert"),
			expected: &Config{
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
			require.NoError(t, component.UnmarshalConfig(sub, cfg))

			assert.NoError(t, component.ValidateConfig(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}
