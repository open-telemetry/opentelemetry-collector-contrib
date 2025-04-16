// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package transformprocessor

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"

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
				ErrorMode: ottl.PropagateError,
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
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "with_conditions"),
			expected: &Config{
				ErrorMode: ottl.PropagateError,
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
				MetricStatements: []common.ContextStatements{},
				LogStatements:    []common.ContextStatements{},
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
			id: component.NewIDWithName(metadata.Type, "bad_syntax_multi_signal"),
			errors: []error{
				errors.New("unexpected token \"where\""),
				errors.New("unexpected token \"attributes\""),
				errors.New("unexpected token \"none\""),
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "structured_configuration_with_path_context"),
			expected: &Config{
				ErrorMode: ottl.PropagateError,
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
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "structured_configuration_with_inferred_context"),
			expected: &Config{
				ErrorMode: ottl.PropagateError,
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
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "flat_configuration"),
			expected: &Config{
				ErrorMode: ottl.PropagateError,
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
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.id.Name(), func(t *testing.T) {
			cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
			assert.NoError(t, err)

			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			assert.NoError(t, err)
			assert.NoError(t, sub.Unmarshal(cfg))

			if tt.expected == nil {
				err = xconfmap.Validate(cfg)
				assert.Error(t, err)

				if len(tt.errors) > 0 {
					for _, expectedErr := range tt.errors {
						assert.ErrorContains(t, err, expectedErr.Error())
					}
				}
			} else {
				assert.NoError(t, xconfmap.Validate(cfg))
				assert.Equal(t, tt.expected, cfg)
			}
		})
	}
}

func Test_UnknownContextID(t *testing.T) {
	id := component.NewIDWithName(metadata.Type, "unknown_context")

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	assert.NoError(t, err)

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(id.String())
	assert.NoError(t, err)
	assert.Error(t, sub.Unmarshal(cfg))
}

func Test_UnknownErrorMode(t *testing.T) {
	id := component.NewIDWithName(metadata.Type, "unknown_error_mode")

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	assert.NoError(t, err)

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(id.String())
	assert.NoError(t, err)
	assert.Error(t, sub.Unmarshal(cfg))
}

func Test_MixedConfigurationStyles(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	assert.NoError(t, err)

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "mixed_configuration_styles").String())
	assert.NoError(t, err)
	assert.ErrorContains(t, sub.Unmarshal(cfg), "configuring multiple configuration styles is not supported")
}
