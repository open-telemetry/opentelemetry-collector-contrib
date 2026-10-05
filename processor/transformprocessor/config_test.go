// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package transformprocessor

import (
	"errors"
	"path/filepath"
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
				TraceStatements: []common.ContextStatements{
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
				MetricStatements: []common.ContextStatements{
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
				LogStatements: []common.ContextStatements{
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
				ProfileStatements: []common.ContextStatements{
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
				TraceStatements: []common.ContextStatements{
					{
						Context:    "span",
						Conditions: []string{`attributes["http.path"] == "/animal"`},
						Statements: []string{
							`set(name, "bear")`,
						},
					},
				},
				MetricStatements: []common.ContextStatements{
					{
						Context:    "datapoint",
						Conditions: []string{`attributes["http.path"] == "/animal"`},
						Statements: []string{
							`set(metric.name, "bear")`,
						},
					},
				},
				LogStatements: []common.ContextStatements{
					{
						Context:    "log",
						Conditions: []string{`attributes["http.path"] == "/animal"`},
						Statements: []string{
							`set(body, "bear")`,
						},
					},
				},
				ProfileStatements: []common.ContextStatements{
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
				TraceStatements: []common.ContextStatements{
					{
						Context: "resource",
						Statements: []string{
							`set(attributes["name"], "bear")`,
						},
					},
				},
				MetricStatements:  []common.ContextStatements{},
				LogStatements:     []common.ContextStatements{},
				ProfileStatements: []common.ContextStatements{},
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
				errors.New("invalid syntax at 1:18 near `where attr`"),
				errors.New("invalid syntax at 1:18 near `attributes`"),
				errors.New("invalid syntax at 1:18 near `none"),
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "structured_configuration_with_path_context"),
			expected: &Config{
				ErrorMode: ottl.IgnoreError,
				TraceStatements: []common.ContextStatements{
					{
						Context:    "span",
						Statements: []string{`set(span.name, "bear") where span.attributes["http.path"] == "/animal"`},
					},
				},
				MetricStatements: []common.ContextStatements{
					{
						Context:    "metric",
						Statements: []string{`set(metric.name, "bear") where resource.attributes["http.path"] == "/animal"`},
					},
				},
				LogStatements: []common.ContextStatements{
					{
						Context:    "log",
						Statements: []string{`set(log.body, "bear") where log.attributes["http.path"] == "/animal"`},
					},
				},
				ProfileStatements: []common.ContextStatements{
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
				TraceStatements: []common.ContextStatements{
					{
						Statements: []string{
							`set(span.name, "bear") where span.attributes["http.path"] == "/animal"`,
							`set(resource.attributes["name"], "bear")`,
						},
					},
				},
				MetricStatements: []common.ContextStatements{
					{
						Statements: []string{
							`set(metric.name, "bear") where resource.attributes["http.path"] == "/animal"`,
							`set(resource.attributes["name"], "bear")`,
						},
					},
				},
				LogStatements: []common.ContextStatements{
					{
						Statements: []string{
							`set(log.body, "bear") where log.attributes["http.path"] == "/animal"`,
							`set(resource.attributes["name"], "bear")`,
						},
					},
				},
				ProfileStatements: []common.ContextStatements{
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
				TraceStatements: []common.ContextStatements{
					{
						Statements: []string{
							`set(span.name, "bear") where span.attributes["http.path"] == "/animal"`,
							`set(resource.attributes["name"], "bear")`,
						},
					},
				},
				MetricStatements: []common.ContextStatements{
					{
						Statements: []string{
							`set(metric.name, "bear") where resource.attributes["http.path"] == "/animal"`,
							`set(resource.attributes["name"], "bear")`,
						},
					},
				},
				LogStatements: []common.ContextStatements{
					{
						Statements: []string{
							`set(log.body, "bear") where log.attributes["http.path"] == "/animal"`,
							`set(resource.attributes["name"], "bear")`,
						},
					},
				},
				ProfileStatements: []common.ContextStatements{
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
				TraceStatements: []common.ContextStatements{
					{
						Statements: []string{`set(resource.attributes["name"], "propagate")`},
						ErrorMode:  ottl.PropagateError,
					},
					{
						Statements: []string{`set(resource.attributes["name"], "ignore")`},
						ErrorMode:  "",
					},
				},
				MetricStatements: []common.ContextStatements{
					{
						Statements: []string{`set(resource.attributes["name"], "silent")`},
						ErrorMode:  ottl.SilentError,
					},
					{
						Statements: []string{`set(resource.attributes["name"], "ignore")`},
						ErrorMode:  "",
					},
				},
				LogStatements: []common.ContextStatements{
					{
						Statements: []string{`set(resource.attributes["name"], "propagate")`},
						ErrorMode:  ottl.PropagateError,
					},
					{
						Statements: []string{`set(resource.attributes["name"], "ignore")`},
						ErrorMode:  "",
					},
				},
				ProfileStatements: []common.ContextStatements{
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
				TraceStatements: []common.ContextStatements{
					{
						SharedCache: true,
						Statements:  []string{`set(resource.attributes["name"], "bear")`},
					},
					{
						SharedCache: true,
						Statements:  []string{`set(resource.attributes["copy"], resource.attributes["name"])`},
					},
				},
				MetricStatements: []common.ContextStatements{
					{
						SharedCache: true,
						Statements:  []string{`set(resource.attributes["name"], "bear")`},
					},
				},
				LogStatements: []common.ContextStatements{
					{
						SharedCache: true,
						Statements:  []string{`set(resource.attributes["name"], "bear")`},
					},
				},
				ProfileStatements: []common.ContextStatements{},
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

func Test_LoadConfig_Flatten(t *testing.T) {
	setFlattenLogsFeatureGate(t, true)

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	cfg := NewFactory().CreateDefaultConfig()
	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "flatten").String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))
	require.NoError(t, confmap.Validate(cfg))

	assert.Equal(t, []common.ContextStatements{
		{
			Flatten:    true,
			Statements: []string{`set(resource.attributes["host.name"], log.attributes["host.name"])`},
		},
		{
			Statements: []string{`set(log.attributes["test"], "pass")`},
		},
	}, cfg.(*Config).LogStatements)
}

func Test_Validate_FlattenRequiresGate(t *testing.T) {
	setFlattenLogsFeatureGate(t, false)

	cfg := NewFactory().CreateDefaultConfig().(*Config)
	cfg.LogStatements = []common.ContextStatements{
		{
			Flatten:    true,
			Statements: []string{`set(resource.attributes["host.name"], log.attributes["host.name"])`},
		},
	}
	assert.ErrorIs(t, cfg.Validate(), errFlatLogsGateDisabled)
}

func Test_Validate_Flatten(t *testing.T) {
	setFlattenLogsFeatureGate(t, true)

	tests := []struct {
		name      string
		configure func(*Config)
		wantErr   string
	}{
		{
			name: "inferred log context",
			configure: func(c *Config) {
				c.LogStatements = []common.ContextStatements{
					{Flatten: true, Statements: []string{`set(resource.attributes["host.name"], log.attributes["host.name"])`}},
				}
			},
		},
		{
			name: "explicit log context",
			configure: func(c *Config) {
				c.LogStatements = []common.ContextStatements{
					{Context: common.Log, Flatten: true, Statements: []string{`set(resource.attributes["host.name"], attributes["host.name"])`}},
				}
			},
		},
		{
			name: "explicit resource context",
			configure: func(c *Config) {
				c.LogStatements = []common.ContextStatements{
					{Context: common.Resource, Flatten: true, Statements: []string{`set(attributes["host.name"], "localhost")`}},
				}
			},
			wantErr: `'flatten' is only supported for statement groups in the "log" context, got "resource"`,
		},
		{
			name: "inferred resource context",
			configure: func(c *Config) {
				c.LogStatements = []common.ContextStatements{
					{Flatten: true, Statements: []string{`set(resource.attributes["host.name"], "localhost")`}},
				}
			},
			wantErr: `'flatten' is only supported for statement groups in the "log" context, got "resource"`,
		},
		{
			name: "inferred scope context",
			configure: func(c *Config) {
				c.LogStatements = []common.ContextStatements{
					{Flatten: true, Statements: []string{`set(scope.attributes["name"], "scope")`}},
				}
			},
			wantErr: `'flatten' is only supported for statement groups in the "log" context, got "scope"`,
		},
		{
			name: "trace statements",
			configure: func(c *Config) {
				c.TraceStatements = []common.ContextStatements{
					{Flatten: true, Statements: []string{`set(resource.attributes["host.name"], span.attributes["host.name"])`}},
				}
			},
			wantErr: errFlattenUnsupportedSignal("trace_statements").Error(),
		},
		{
			name: "metric statements",
			configure: func(c *Config) {
				c.MetricStatements = []common.ContextStatements{
					{Flatten: true, Statements: []string{`set(resource.attributes["host.name"], datapoint.attributes["host.name"])`}},
				}
			},
			wantErr: errFlattenUnsupportedSignal("metric_statements").Error(),
		},
		{
			name: "profile statements",
			configure: func(c *Config) {
				c.ProfileStatements = []common.ContextStatements{
					{Flatten: true, Statements: []string{`set(resource.attributes["host.name"], "localhost")`}},
				}
			},
			wantErr: errFlattenUnsupportedSignal("profile_statements").Error(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := NewFactory().CreateDefaultConfig().(*Config)
			tt.configure(cfg)
			err := cfg.Validate()
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			assert.ErrorContains(t, err, tt.wantErr)
		})
	}
}
