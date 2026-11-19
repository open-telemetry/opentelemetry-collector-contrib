// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package transformprocessor

import (
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/featuregate"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		id       component.ID
		expected component.Config
		errors   []error
	}{
		{
			id: component.NewIDWithName(metadata.Type, ""),
			expected: &Config{
				ErrorMode: ottl.IgnoreError,
				TraceStatements: []ContextStatements{
					{
						Context: "span",
						Statements: []string{
							`set(name, "bear") where attributes["http.path"] == "/animal"`,
							`keep_keys(attributes, ["http.method", "http.path"])`,
						},
					},
					{
						Context: "resource",
						Statements: []string{
							`set(attributes["name"], "bear")`,
						},
					},
				},
				MetricStatements: []ContextStatements{
					{
						Context: "datapoint",
						Statements: []string{
							`set(metric.name, "bear") where attributes["http.path"] == "/animal"`,
							`keep_keys(attributes, ["http.method", "http.path"])`,
						},
					},
					{
						Context: "resource",
						Statements: []string{
							`set(attributes["name"], "bear")`,
						},
					},
				},
				LogStatements: []ContextStatements{
					{
						Context: "log",
						Statements: []string{
							`set(body, "bear") where attributes["http.path"] == "/animal"`,
							`keep_keys(attributes, ["http.method", "http.path"])`,
						},
					},
					{
						Context: "resource",
						Statements: []string{
							`set(attributes["name"], "bear")`,
						},
					},
				},
				ProfileStatements: []ContextStatements{
					{
						Context: "profile",
						Statements: []string{
							`set(original_payload_format, "bear") where original_payload_format == "/animal"`,
						},
					},
					{
						Context: "resource",
						Statements: []string{
							`set(attributes["name"], "bear")`,
						},
					},
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "with_conditions"),
			expected: &Config{
				ErrorMode: ottl.IgnoreError,
				TraceStatements: []ContextStatements{
					{
						Context:    "span",
						Conditions: []string{`attributes["http.path"] == "/animal"`},
						Statements: []string{
							`set(name, "bear")`,
						},
					},
				},
				MetricStatements: []ContextStatements{
					{
						Context:    "datapoint",
						Conditions: []string{`attributes["http.path"] == "/animal"`},
						Statements: []string{
							`set(metric.name, "bear")`,
						},
					},
				},
				LogStatements: []ContextStatements{
					{
						Context:    "log",
						Conditions: []string{`attributes["http.path"] == "/animal"`},
						Statements: []string{
							`set(body, "bear")`,
						},
					},
				},
				ProfileStatements: []ContextStatements{
					{
						Context:    "profile",
						Conditions: []string{`original_payload_format == "/animal"`},
						Statements: []string{
							`set(original_payload_format, "bear")`,
						},
					},
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "ignore_errors"),
			expected: &Config{
				ErrorMode: ottl.IgnoreError,
				TraceStatements: []ContextStatements{
					{
						Context: "resource",
						Statements: []string{
							`set(attributes["name"], "bear")`,
						},
					},
				},
				MetricStatements:  []ContextStatements{},
				LogStatements:     []ContextStatements{},
				ProfileStatements: []ContextStatements{},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "bad_syntax_trace"),
		},
		{
			id: component.NewIDWithName(metadata.Type, "unknown_function_trace"),
		},
		{
			id: component.NewIDWithName(metadata.Type, "bad_syntax_metric"),
		},
		{
			id: component.NewIDWithName(metadata.Type, "unknown_function_metric"),
		},
		{
			id: component.NewIDWithName(metadata.Type, "bad_syntax_log"),
		},
		{
			id: component.NewIDWithName(metadata.Type, "unknown_function_log"),
		},
		{
			id: component.NewIDWithName(metadata.Type, "bad_syntax_profile"),
		},
		{
			id: component.NewIDWithName(metadata.Type, "unknown_function_profile"),
		},
		{
			id: component.NewIDWithName(metadata.Type, "bad_syntax_multi_signal"),
			errors: []error{
				errors.New("1:18: statement has invalid syntax near `where attr`"),
				errors.New("1:18: statement has invalid syntax near `attributes`"),
				errors.New("1:18: statement has invalid syntax near `none"),
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "structured_configuration_with_path_context"),
			expected: &Config{
				ErrorMode: ottl.IgnoreError,
				TraceStatements: []ContextStatements{
					{
						Context:    "span",
						Statements: []string{`set(span.name, "bear") where span.attributes["http.path"] == "/animal"`},
					},
				},
				MetricStatements: []ContextStatements{
					{
						Context:    "metric",
						Statements: []string{`set(metric.name, "bear") where resource.attributes["http.path"] == "/animal"`},
					},
				},
				LogStatements: []ContextStatements{
					{
						Context:    "log",
						Statements: []string{`set(log.body, "bear") where log.attributes["http.path"] == "/animal"`},
					},
				},
				ProfileStatements: []ContextStatements{
					{
						Context:    "profile",
						Statements: []string{`set(profile.original_payload_format, "bear") where profile.original_payload_format == "/animal"`},
					},
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "structured_configuration_with_inferred_context"),
			expected: &Config{
				ErrorMode: ottl.IgnoreError,
				TraceStatements: []ContextStatements{
					{
						Statements: []string{
							`set(span.name, "bear") where span.attributes["http.path"] == "/animal"`,
							`set(resource.attributes["name"], "bear")`,
						},
					},
				},
				MetricStatements: []ContextStatements{
					{
						Statements: []string{
							`set(metric.name, "bear") where resource.attributes["http.path"] == "/animal"`,
							`set(resource.attributes["name"], "bear")`,
						},
					},
				},
				LogStatements: []ContextStatements{
					{
						Statements: []string{
							`set(log.body, "bear") where log.attributes["http.path"] == "/animal"`,
							`set(resource.attributes["name"], "bear")`,
						},
					},
				},
				ProfileStatements: []ContextStatements{
					{
						Statements: []string{
							`set(profile.original_payload_format, "bear") where profile.original_payload_format == "/animal"`,
							`set(resource.attributes["name"], "bear")`,
						},
					},
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "flat_configuration"),
			expected: &Config{
				ErrorMode: ottl.IgnoreError,
				TraceStatements: []ContextStatements{
					{
						Statements: []string{
							`set(span.name, "bear") where span.attributes["http.path"] == "/animal"`,
							`set(resource.attributes["name"], "bear")`,
						},
					},
				},
				MetricStatements: []ContextStatements{
					{
						Statements: []string{
							`set(metric.name, "bear") where resource.attributes["http.path"] == "/animal"`,
							`set(resource.attributes["name"], "bear")`,
						},
					},
				},
				LogStatements: []ContextStatements{
					{
						Statements: []string{
							`set(log.body, "bear") where log.attributes["http.path"] == "/animal"`,
							`set(resource.attributes["name"], "bear")`,
						},
					},
				},
				ProfileStatements: []ContextStatements{
					{
						Statements: []string{
							`set(profile.original_payload_format, "bear") where profile.original_payload_format == "/animal"`,
							`set(resource.attributes["name"], "bear")`,
						},
					},
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "context_statements_error_mode"),
			expected: &Config{
				ErrorMode: ottl.IgnoreError,
				TraceStatements: []ContextStatements{
					{
						Statements: []string{`set(resource.attributes["name"], "propagate")`},
						ErrorMode:  ottl.PropagateError,
					},
					{
						Statements: []string{`set(resource.attributes["name"], "ignore")`},
						ErrorMode:  "",
					},
				},
				MetricStatements: []ContextStatements{
					{
						Statements: []string{`set(resource.attributes["name"], "silent")`},
						ErrorMode:  ottl.SilentError,
					},
					{
						Statements: []string{`set(resource.attributes["name"], "ignore")`},
						ErrorMode:  "",
					},
				},
				LogStatements: []ContextStatements{
					{
						Statements: []string{`set(resource.attributes["name"], "propagate")`},
						ErrorMode:  ottl.PropagateError,
					},
					{
						Statements: []string{`set(resource.attributes["name"], "ignore")`},
						ErrorMode:  "",
					},
				},
				ProfileStatements: []ContextStatements{
					{
						Statements: []string{`set(resource.attributes["name"], "propagate")`},
						ErrorMode:  ottl.PropagateError,
					},
					{
						Statements: []string{`set(resource.attributes["name"], "ignore")`},
						ErrorMode:  "",
					},
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "shared_cache"),
			expected: &Config{
				ErrorMode: ottl.IgnoreError,
				TraceStatements: []ContextStatements{
					{
						SharedCache: true,
						Statements:  []string{`set(resource.attributes["name"], "bear")`},
					},
					{
						SharedCache: true,
						Statements:  []string{`set(resource.attributes["copy"], resource.attributes["name"])`},
					},
				},
				MetricStatements: []ContextStatements{
					{
						SharedCache: true,
						Statements:  []string{`set(resource.attributes["name"], "bear")`},
					},
				},
				LogStatements: []ContextStatements{
					{
						SharedCache: true,
						Statements:  []string{`set(resource.attributes["name"], "bear")`},
					},
				},
				ProfileStatements: []ContextStatements{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.id.Name(), func(t *testing.T) {
			cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
			require.NoError(t, err)

			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			if tt.expected == nil {
				err = confmap.Validate(cfg)
				assert.Error(t, err)

				if len(tt.errors) > 0 {
					for _, expectedErr := range tt.errors {
						assert.ErrorContains(t, err, expectedErr.Error())
					}
				}
			} else {
				require.NoError(t, confmap.Validate(cfg))
				assert.EqualExportedValues(t, tt.expected, cfg)
				assertConfigContainsDefaultFunctions(t, *cfg.(*Config))
			}
		})
	}
}

func Test_UnknownContextID(t *testing.T) {
	id := component.NewIDWithName(metadata.Type, "unknown_context")

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(id.String())
	require.NoError(t, err)
	assert.Error(t, sub.Unmarshal(cfg))
}

func Test_UnknownErrorMode(t *testing.T) {
	id := component.NewIDWithName(metadata.Type, "unknown_error_mode")

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(id.String())
	require.NoError(t, err)
	assert.Error(t, sub.Unmarshal(cfg))
}

func Test_MixedConfigurationStyles(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "mixed_configuration_styles").String())
	require.NoError(t, err)
	assert.ErrorContains(t, sub.Unmarshal(cfg), "configuring multiple configuration styles is not supported")
}

func Test_EmptyStatementListItem(t *testing.T) {
	t.Parallel()

	for _, fieldName := range []string{
		"trace_statements",
		"metric_statements",
		"log_statements",
		"profile_statements",
	} {
		t.Run(fieldName, func(t *testing.T) {
			t.Parallel()

			cfg := NewFactory().CreateDefaultConfig()
			conf := confmap.NewFromStringMap(map[string]any{
				fieldName: []any{nil},
			})

			require.ErrorContains(t, conf.Unmarshal(cfg), "invalid "+fieldName+" item: empty statement list items are not supported")
		})
	}
}

func setFlattenLogsFeatureGate(t *testing.T, enabled bool) {
	original := metadata.TransformFlattenLogsFeatureGate.IsEnabled()
	require.NoError(t, featuregate.GlobalRegistry().Set(metadata.TransformFlattenLogsFeatureGate.ID(), enabled))
	t.Cleanup(func() {
		require.NoError(t, featuregate.GlobalRegistry().Set(metadata.TransformFlattenLogsFeatureGate.ID(), original))
	})
}

func Test_LoadConfig_FlattenLog(t *testing.T) {
	setFlattenLogsFeatureGate(t, true)

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	cfg := NewFactory().CreateDefaultConfig()
	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "flatten").String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))
	require.NoError(t, confmap.Validate(cfg))

	assert.True(t, cfg.(*Config).Flatten.Log)
}

func TestContextID_UnmarshalText(t *testing.T) {
	valid := []ContextID{Resource, Scope, Span, SpanEvent, Metric, DataPoint, Exemplar, Log, Profile}
	for _, want := range valid {
		t.Run(string(want), func(t *testing.T) {
			var got ContextID
			require.NoError(t, got.UnmarshalText([]byte(want)))
			assert.Equal(t, want, got)
		})
	}

	t.Run("normalizes case", func(t *testing.T) {
		var got ContextID
		require.NoError(t, got.UnmarshalText([]byte("SPAN")))
		assert.Equal(t, Span, got)
	})

	t.Run("rejects unknown context", func(t *testing.T) {
		var got ContextID
		err := got.UnmarshalText([]byte("nonsense"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown context")
	})
}

func TestContextStatements_GetStatements(t *testing.T) {
	cs := ContextStatements{Statements: []string{"a", "b"}}
	assert.Equal(t, []string{"a", "b"}, cs.GetStatements())
}

func TestContextStatements_MatchesCommon(t *testing.T) {
	public := reflect.TypeFor[ContextStatements]()
	internal := reflect.TypeFor[common.ContextStatements]()
	require.Equal(t, internal.NumField(), public.NumField())
	for i := range public.NumField() {
		assert.Equal(t, internal.Field(i).Name, public.Field(i).Name)
		assert.Equal(t, internal.Field(i).Tag, public.Field(i).Tag)
	}
}

func TestContextStatements_toCommon(t *testing.T) {
	cs := ContextStatements{
		Context:     Span,
		Conditions:  []string{"c"},
		Statements:  []string{"s"},
		ErrorMode:   ottl.PropagateError,
		SharedCache: true,
	}
	expected := common.ContextStatements{
		Context:     common.Span,
		Conditions:  []string{"c"},
		Statements:  []string{"s"},
		ErrorMode:   ottl.PropagateError,
		SharedCache: true,
	}
	assert.Equal(t, expected, cs.toCommon())
	assert.Equal(t, []common.ContextStatements{expected}, toCommonContextStatements([]ContextStatements{cs}))
}
