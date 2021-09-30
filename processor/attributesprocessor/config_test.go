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
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configtest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/attraction"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/processor/filterconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/processor/filterset"
)

func TestLoadingConfig(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Processors[typeStr] = factory
	cfg, err := configtest.LoadConfigAndValidate(path.Join(".", "testdata", "config.yaml"), factories)
	assert.NoError(t, err)
	require.NotNil(t, cfg)

	p0 := cfg.Processors[config.NewComponentIDWithName(typeStr, "insert")]
	assert.Equal(t, p0, &Config{
		ProcessorSettings: config.NewProcessorSettings(config.NewComponentIDWithName(typeStr, "insert")),
		Settings: attraction.Settings{
			Actions: []attraction.ActionKeyValue{
				{Key: "attribute1", Value: 123, Action: attraction.INSERT},
				{Key: "string key", FromAttribute: "anotherkey", Action: attraction.INSERT},
			},
		},
	})

	p1 := cfg.Processors[config.NewComponentIDWithName(typeStr, "update")]
	assert.Equal(t, p1, &Config{
		ProcessorSettings: config.NewProcessorSettings(config.NewComponentIDWithName(typeStr, "update")),
		Settings: attraction.Settings{
			Actions: []attraction.ActionKeyValue{
				{Key: "boo", FromAttribute: "foo", Action: attraction.UPDATE},
				{Key: "db.secret", Value: "redacted", Action: attraction.UPDATE},
			},
		},
	})

	p2 := cfg.Processors[config.NewComponentIDWithName(typeStr, "upsert")]
	assert.Equal(t, p2, &Config{
		ProcessorSettings: config.NewProcessorSettings(config.NewComponentIDWithName(typeStr, "upsert")),
		Settings: attraction.Settings{
			Actions: []attraction.ActionKeyValue{
				{Key: "region", Value: "planet-earth", Action: attraction.UPSERT},
				{Key: "new_user_key", FromAttribute: "user_key", Action: attraction.UPSERT},
			},
		},
	})

	p3 := cfg.Processors[config.NewComponentIDWithName(typeStr, "delete")]
	assert.Equal(t, p3, &Config{
		ProcessorSettings: config.NewProcessorSettings(config.NewComponentIDWithName(typeStr, "delete")),
		Settings: attraction.Settings{
			Actions: []attraction.ActionKeyValue{
				{Key: "credit_card", Action: attraction.DELETE},
				{Key: "duplicate_key", Action: attraction.DELETE},
			},
		},
	})

	p4 := cfg.Processors[config.NewComponentIDWithName(typeStr, "hash")]
	assert.Equal(t, p4, &Config{
		ProcessorSettings: config.NewProcessorSettings(config.NewComponentIDWithName(typeStr, "hash")),
		Settings: attraction.Settings{
			Actions: []attraction.ActionKeyValue{
				{Key: "user.email", Action: attraction.HASH},
			},
		},
	})

	p5 := cfg.Processors[config.NewComponentIDWithName(typeStr, "excludemulti")]
	assert.Equal(t, p5, &Config{
		ProcessorSettings: config.NewProcessorSettings(config.NewComponentIDWithName(typeStr, "excludemulti")),
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
	})

	p6 := cfg.Processors[config.NewComponentIDWithName(typeStr, "includeservices")]
	assert.Equal(t, p6, &Config{
		ProcessorSettings: config.NewProcessorSettings(config.NewComponentIDWithName(typeStr, "includeservices")),
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
	})

	p7 := cfg.Processors[config.NewComponentIDWithName(typeStr, "selectiveprocessing")]
	assert.Equal(t, p7, &Config{
		ProcessorSettings: config.NewProcessorSettings(config.NewComponentIDWithName(typeStr, "selectiveprocessing")),
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
	})

	p8 := cfg.Processors[config.NewComponentIDWithName(typeStr, "complex")]
	assert.Equal(t, p8, &Config{
		ProcessorSettings: config.NewProcessorSettings(config.NewComponentIDWithName(typeStr, "complex")),
		Settings: attraction.Settings{
			Actions: []attraction.ActionKeyValue{
				{Key: "operation", Value: "default", Action: attraction.INSERT},
				{Key: "svc.operation", FromAttribute: "operation", Action: attraction.UPSERT},
				{Key: "operation", Action: attraction.DELETE},
			},
		},
	})

	p9 := cfg.Processors[config.NewComponentIDWithName(typeStr, "example")]
	assert.Equal(t, p9, &Config{
		ProcessorSettings: config.NewProcessorSettings(config.NewComponentIDWithName(typeStr, "example")),
		Settings: attraction.Settings{
			Actions: []attraction.ActionKeyValue{
				{Key: "db.table", Action: attraction.DELETE},
				{Key: "redacted_span", Value: true, Action: attraction.UPSERT},
				{Key: "copy_key", FromAttribute: "key_original", Action: attraction.UPDATE},
				{Key: "account_id", Value: 2245, Action: attraction.INSERT},
				{Key: "account_password", Action: attraction.DELETE},
			},
		},
	})

	p10 := cfg.Processors[config.NewComponentIDWithName(typeStr, "regexp")]
	assert.Equal(t, p10, &Config{
		ProcessorSettings: config.NewProcessorSettings(config.NewComponentIDWithName(typeStr, "regexp")),
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
	})

}
