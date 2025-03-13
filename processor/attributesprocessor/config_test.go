// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package attributesprocessor

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/attraction"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/attributesprocessor/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		id       component.ID
		expected component.Config
	}{
		{
			id: component.NewIDWithName(metadata.Type, "insert"),
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
			id: component.NewIDWithName(metadata.Type, "update"),
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
			id: component.NewIDWithName(metadata.Type, "upsert"),
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
			id: component.NewIDWithName(metadata.Type, "delete"),
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
			id: component.NewIDWithName(metadata.Type, "hash"),
			expected: &Config{
				Settings: attraction.Settings{
					Actions: []attraction.ActionKeyValue{
						{Key: "user.email", Action: attraction.HASH},
					},
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "excludemulti"),
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
			id: component.NewIDWithName(metadata.Type, "includeservices"),
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
			id: component.NewIDWithName(metadata.Type, "selectiveprocessing"),
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
			id: component.NewIDWithName(metadata.Type, "complex"),
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
			id: component.NewIDWithName(metadata.Type, "example"),
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
			id: component.NewIDWithName(metadata.Type, "regexp"),
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
			id: component.NewIDWithName(metadata.Type, "convert"),
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
			require.NoError(t, sub.Unmarshal(cfg))

			assert.NoError(t, xconfmap.Validate(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestSpanConfigUsedWithmetrics(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub("attributes/servicesmetrics")

	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))

	assert.NoError(t, xconfmap.Validate(cfg))

	sink := consumertest.MetricsSink{}

	_, err = NewFactory().CreateMetrics(context.Background(), processortest.NewNopSettings(metadata.Type), cfg, &sink)
	require.Error(t, err)
}
