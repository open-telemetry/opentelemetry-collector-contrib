// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package transformprocessor

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		id       component.ID
		expected component.Config
		errorLen int
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
			id:       component.NewIDWithName(metadata.Type, "bad_syntax_multi_signal"),
			errorLen: 3,
		},
	}
	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
			assert.NoError(t, err)

			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			assert.NoError(t, err)
			assert.NoError(t, sub.Unmarshal(cfg))

			if tt.expected == nil {
				err = component.ValidateConfig(cfg)
				assert.Error(t, err)

				if tt.errorLen > 0 {
					assert.Len(t, multierr.Errors(err), tt.errorLen)
				}

				return
			}
			assert.NoError(t, component.ValidateConfig(cfg))
			assert.Equal(t, tt.expected, cfg)
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
