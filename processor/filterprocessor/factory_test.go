// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filterprocessor

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.opentelemetry.io/collector/processor/xprocessor"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottldatapoint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlprofile"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspanevent"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor/internal/metadata"
)

func TestType(t *testing.T) {
	factory := NewFactory()
	pType := factory.Type()

	assert.Equal(t, pType, metadata.Type)
}

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.EqualExportedValues(t, &Config{
		ErrorMode: ottl.PropagateError,
	}, cfg)
	assertConfigContainsDefaultFunctions(t, *cfg.(*Config))
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestCreateProcessors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		configName string
		succeed    bool
	}{
		{
			configName: "config_regexp.yaml",
			succeed:    true,
		}, {
			configName: "config_strict.yaml",
			succeed:    true,
		}, {
			configName: "config_invalid.yaml",
			succeed:    false,
		}, {
			configName: "config_logs_strict.yaml",
			succeed:    true,
		}, {
			configName: "config_logs_regexp.yaml",
			succeed:    true,
		}, {
			configName: "config_logs_record_attributes_strict.yaml",
			succeed:    true,
		}, {
			configName: "config_logs_record_attributes_regexp.yaml",
			succeed:    true,
		}, {
			configName: "config_traces.yaml",
			succeed:    true,
		}, {
			configName: "config_traces_invalid.yaml",
			succeed:    false,
		}, {
			configName: "config_profiles.yaml",
			succeed:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.configName, func(t *testing.T) {
			cm, err := confmaptest.LoadConf(filepath.Join("testdata", tt.configName))
			require.NoError(t, err)

			for k := range cm.ToStringMap() {
				// Check if all processor variations that are defined in test config can be actually created
				factory := NewFactory()
				cfg := factory.CreateDefaultConfig()

				sub, err := cm.Sub(k)
				require.NoError(t, err)
				require.NoError(t, sub.Unmarshal(cfg))

				tp, tErr := factory.CreateTraces(
					t.Context(),
					processortest.NewNopSettings(metadata.Type),
					cfg, consumertest.NewNop(),
				)
				mp, mErr := factory.CreateMetrics(
					t.Context(),
					processortest.NewNopSettings(metadata.Type),
					cfg,
					consumertest.NewNop(),
				)
				if strings.Contains(tt.configName, "traces") {
					assert.Equal(t, tt.succeed, tp != nil)
					assert.Equal(t, tt.succeed, tErr == nil)

					assert.NotNil(t, mp)
					assert.NoError(t, mErr)
				} else {
					// Should not break configs with no trace data
					assert.NotNil(t, tp)
					assert.NoError(t, tErr)

					assert.Equal(t, tt.succeed, mp != nil)
					assert.Equal(t, tt.succeed, mErr == nil)
				}
			}
		})
	}
}

func createTestFuncFactory[K any](name string) ottl.Factory[K] {
	type TestFuncArguments[K any] struct{}
	createFunc := func(_ ottl.FunctionContext, _ ottl.Arguments) (ottl.ExprFunc[K], error) {
		return func(_ context.Context, _ K) (any, error) {
			return nil, nil
		}, nil
	}
	return ottl.NewFactory(name, &TestFuncArguments[K]{}, createFunc)
}

func Test_FactoryWithFunctions_CreateTraces(t *testing.T) {
	type testCase struct {
		name           string
		conditions     TraceFilters
		factoryOptions []FactoryOption
		wantErrorWith  string
	}

	tests := []testCase{
		{
			name: "with span functions : statement with added span func",
			conditions: TraceFilters{
				SpanConditions: []string{
					`TestSpanFunc() and IsBool(true)`,
				},
			},
			factoryOptions: []FactoryOption{
				WithSpanFunctions(DefaultSpanFunctions()),
				WithSpanFunctions([]ottl.Factory[ottlspan.TransformContext]{createTestFuncFactory[ottlspan.TransformContext]("TestSpanFunc")}),
				WithSpanEventFunctions(DefaultSpanEventFunctions()),
			},
		},
		{
			name: "with span functions : statement with missing span func",
			conditions: TraceFilters{
				SpanConditions: []string{
					`TestSpanFunc() and IsBool(true)`,
				},
			},
			wantErrorWith: `undefined function "TestSpanFunc"`,
			factoryOptions: []FactoryOption{
				WithSpanFunctions(DefaultSpanFunctions()),
				WithSpanEventFunctions(DefaultSpanEventFunctions()),
			},
		},
		{
			name: "with span functions : only custom functions",
			conditions: TraceFilters{
				SpanConditions: []string{
					`TestSpanFunc()`,
				},
			},
			factoryOptions: []FactoryOption{
				WithSpanFunctions([]ottl.Factory[ottlspan.TransformContext]{createTestFuncFactory[ottlspan.TransformContext]("TestSpanFunc")}),
			},
		},
		{
			name: "with span functions : missing default functions",
			conditions: TraceFilters{
				SpanConditions: []string{
					`TestSpanFunc() and IsBool(true)`,
				},
			},
			wantErrorWith: `undefined function "IsBool"`,
			factoryOptions: []FactoryOption{
				WithSpanFunctions([]ottl.Factory[ottlspan.TransformContext]{createTestFuncFactory[ottlspan.TransformContext]("TestSpanFunc")}),
			},
		},
		{
			name: "with span event functions : statement with added span event func",
			conditions: TraceFilters{
				SpanEventConditions: []string{
					`TestSpanEventFunc() and IsBool(true)`,
				},
			},
			factoryOptions: []FactoryOption{
				WithSpanFunctions(DefaultSpanFunctions()),
				WithSpanEventFunctions(DefaultSpanEventFunctions()),
				WithSpanEventFunctions([]ottl.Factory[ottlspanevent.TransformContext]{createTestFuncFactory[ottlspanevent.TransformContext]("TestSpanEventFunc")}),
			},
		},
		{
			name: "with span event functions : statement with missing span event func",
			conditions: TraceFilters{
				SpanEventConditions: []string{
					`TestSpanEventFunc() and IsBool(true)`,
				},
			},
			wantErrorWith: `undefined function "TestSpanEventFunc"`,
			factoryOptions: []FactoryOption{
				WithSpanFunctions(DefaultSpanFunctions()),
				WithSpanEventFunctions(DefaultSpanEventFunctions()),
			},
		},
		{
			name: "with span event functions : only custom functions",
			conditions: TraceFilters{
				SpanEventConditions: []string{
					`TestSpanEventFunc()`,
				},
			},
			factoryOptions: []FactoryOption{
				WithSpanEventFunctions([]ottl.Factory[ottlspanevent.TransformContext]{createTestFuncFactory[ottlspanevent.TransformContext]("TestSpanEventFunc")}),
			},
		},
		{
			name: "with span event functions : missing default functions",
			conditions: TraceFilters{
				SpanEventConditions: []string{
					`TestSpanEventFunc() and IsBool(true)`,
				},
			},
			wantErrorWith: `undefined function "IsBool"`,
			factoryOptions: []FactoryOption{
				WithSpanEventFunctions([]ottl.Factory[ottlspanevent.TransformContext]{createTestFuncFactory[ottlspanevent.TransformContext]("TestSpanEventFunc")}),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := NewFactoryWithOptions(tt.factoryOptions...)
			cfg := factory.CreateDefaultConfig()
			oCfg := cfg.(*Config)
			oCfg.ErrorMode = ottl.IgnoreError
			oCfg.Traces = tt.conditions

			_, err := factory.CreateTraces(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
			if tt.wantErrorWith != "" {
				if err == nil {
					t.Errorf("expected error containing '%s', got: <nil>", tt.wantErrorWith)
				}
				assert.Contains(t, err.Error(), tt.wantErrorWith)
				return
			}
			require.NoError(t, err)
		})
	}
}

func Test_FactoryWithFunctions_CreateLogs(t *testing.T) {
	type testCase struct {
		name           string
		conditions     LogFilters
		factoryOptions []FactoryOption
		wantErrorWith  string
	}

	tests := []testCase{
		{
			name: "with log functions : statement with added log func",
			conditions: LogFilters{
				LogConditions: []string{
					`TestLogFunc() and IsBool(true)`,
				},
			},
			factoryOptions: []FactoryOption{
				WithLogFunctions(DefaultLogFunctions()),
				WithLogFunctions([]ottl.Factory[ottllog.TransformContext]{createTestFuncFactory[ottllog.TransformContext]("TestLogFunc")}),
			},
		},
		{
			name: "with log functions : statement with missing log func",
			conditions: LogFilters{
				LogConditions: []string{
					`TestLogFunc() and IsBool(true)`,
				},
			},
			wantErrorWith: `undefined function "TestLogFunc"`,
			factoryOptions: []FactoryOption{
				WithLogFunctions(DefaultLogFunctions()),
			},
		},
		{
			name: "with log functions : only custom functions",
			conditions: LogFilters{
				LogConditions: []string{
					`TestLogFunc()`,
				},
			},
			factoryOptions: []FactoryOption{
				WithLogFunctions([]ottl.Factory[ottllog.TransformContext]{createTestFuncFactory[ottllog.TransformContext]("TestLogFunc")}),
			},
		},
		{
			name: "with log functions : missing default functions",
			conditions: LogFilters{
				LogConditions: []string{
					`TestLogFunc() and IsBool(true)`,
				},
			},
			wantErrorWith: `undefined function "IsBool"`,
			factoryOptions: []FactoryOption{
				WithLogFunctions([]ottl.Factory[ottllog.TransformContext]{createTestFuncFactory[ottllog.TransformContext]("TestLogFunc")}),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := NewFactoryWithOptions(tt.factoryOptions...)
			cfg := factory.CreateDefaultConfig()
			oCfg := cfg.(*Config)
			oCfg.ErrorMode = ottl.IgnoreError
			oCfg.Logs = tt.conditions

			_, err := factory.CreateLogs(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
			if tt.wantErrorWith != "" {
				if err == nil {
					t.Errorf("expected error containing '%s', got: <nil>", tt.wantErrorWith)
				}
				assert.Contains(t, err.Error(), tt.wantErrorWith)
				return
			}
			require.NoError(t, err)
		})
	}
}

func Test_FactoryWithFunctions_CreateMetrics(t *testing.T) {
	type testCase struct {
		name           string
		conditions     MetricFilters
		factoryOptions []FactoryOption
		wantErrorWith  string
	}

	tests := []testCase{
		{
			name: "with metric functions : statement with added metric func",
			conditions: MetricFilters{
				MetricConditions: []string{
					`TestMetricFunc() and IsBool(true)`,
				},
			},
			factoryOptions: []FactoryOption{
				WithMetricFunctions(DefaultMetricFunctions()),
				WithMetricFunctions([]ottl.Factory[ottlmetric.TransformContext]{createTestFuncFactory[ottlmetric.TransformContext]("TestMetricFunc")}),
				WithDataPointFunctions(DefaultDataPointFunctions()),
			},
		},
		{
			name: "with metric functions : statement with missing metric func",
			conditions: MetricFilters{
				MetricConditions: []string{
					`TestMetricFunc() and IsBool(true)`,
				},
			},
			wantErrorWith: `undefined function "TestMetricFunc"`,
			factoryOptions: []FactoryOption{
				WithMetricFunctions(DefaultMetricFunctions()),
				WithDataPointFunctions(DefaultDataPointFunctions()),
			},
		},
		{
			name: "with metric functions : only custom functions",
			conditions: MetricFilters{
				MetricConditions: []string{
					`TestMetricFunc()`,
				},
			},
			factoryOptions: []FactoryOption{
				WithMetricFunctions([]ottl.Factory[ottlmetric.TransformContext]{createTestFuncFactory[ottlmetric.TransformContext]("TestMetricFunc")}),
			},
		},
		{
			name: "with metric functions : missing default functions",
			conditions: MetricFilters{
				MetricConditions: []string{
					`TestMetricFunc() and IsBool(true)`,
				},
			},
			wantErrorWith: `undefined function "IsBool"`,
			factoryOptions: []FactoryOption{
				WithMetricFunctions([]ottl.Factory[ottlmetric.TransformContext]{createTestFuncFactory[ottlmetric.TransformContext]("TestMetricFunc")}),
			},
		},
		{
			name: "with datapoint functions : statement with added datapoint func",
			conditions: MetricFilters{
				DataPointConditions: []string{
					`TestDataPointFunc() and IsBool(true)`,
				},
			},
			factoryOptions: []FactoryOption{
				WithMetricFunctions(DefaultMetricFunctions()),
				WithDataPointFunctions(DefaultDataPointFunctions()),
				WithDataPointFunctions([]ottl.Factory[ottldatapoint.TransformContext]{createTestFuncFactory[ottldatapoint.TransformContext]("TestDataPointFunc")}),
			},
		},
		{
			name: "with datapoint functions : statement with missing datapoint func",
			conditions: MetricFilters{
				DataPointConditions: []string{
					`TestDataPointFunc() and IsBool(true)`,
				},
			},
			wantErrorWith: `undefined function "TestDataPointFunc"`,
			factoryOptions: []FactoryOption{
				WithMetricFunctions(DefaultMetricFunctions()),
				WithDataPointFunctions(DefaultDataPointFunctions()),
			},
		},
		{
			name: "with datapoint functions : only custom functions",
			conditions: MetricFilters{
				DataPointConditions: []string{
					`TestDataPointFunc()`,
				},
			},
			factoryOptions: []FactoryOption{
				WithDataPointFunctions([]ottl.Factory[ottldatapoint.TransformContext]{createTestFuncFactory[ottldatapoint.TransformContext]("TestDataPointFunc")}),
			},
		},
		{
			name: "with datapoint functions : missing default functions",
			conditions: MetricFilters{
				DataPointConditions: []string{
					`TestDataPointFunc() and IsBool(true)`,
				},
			},
			wantErrorWith: `undefined function "IsBool"`,
			factoryOptions: []FactoryOption{
				WithDataPointFunctions([]ottl.Factory[ottldatapoint.TransformContext]{createTestFuncFactory[ottldatapoint.TransformContext]("TestDataPointFunc")}),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := NewFactoryWithOptions(tt.factoryOptions...)
			cfg := factory.CreateDefaultConfig()
			oCfg := cfg.(*Config)
			oCfg.ErrorMode = ottl.IgnoreError
			oCfg.Metrics = tt.conditions

			_, err := factory.CreateMetrics(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
			if tt.wantErrorWith != "" {
				if err == nil {
					t.Errorf("expected error containing '%s', got: <nil>", tt.wantErrorWith)
				}
				assert.Contains(t, err.Error(), tt.wantErrorWith)
				return
			}
			require.NoError(t, err)
		})
	}
}

func Test_FactoryWithFunctions_CreateProfiles(t *testing.T) {
	type testCase struct {
		name           string
		conditions     ProfileFilters
		factoryOptions []FactoryOption
		wantErrorWith  string
	}

	tests := []testCase{
		{
			name: "with profile functions : statement with added profile func",
			conditions: ProfileFilters{
				ProfileConditions: []string{
					`TestProfileFunc() and IsBool(true)`,
				},
			},
			factoryOptions: []FactoryOption{
				WithProfileFunctions(DefaultProfileFunctions()),
				WithProfileFunctions([]ottl.Factory[ottlprofile.TransformContext]{createTestFuncFactory[ottlprofile.TransformContext]("TestProfileFunc")}),
			},
		},
		{
			name: "with profile functions : statement with missing profile func",
			conditions: ProfileFilters{
				ProfileConditions: []string{
					`TestProfileFunc() and IsBool(true)`,
				},
			},
			wantErrorWith: `undefined function "TestProfileFunc"`,
			factoryOptions: []FactoryOption{
				WithProfileFunctions(DefaultProfileFunctions()),
			},
		},
		{
			name: "with profile functions : only custom functions",
			conditions: ProfileFilters{
				ProfileConditions: []string{
					`TestProfileFunc()`,
				},
			},
			factoryOptions: []FactoryOption{
				WithProfileFunctions([]ottl.Factory[ottlprofile.TransformContext]{createTestFuncFactory[ottlprofile.TransformContext]("TestProfileFunc")}),
			},
		},
		{
			name: "with profile functions : missing default functions",
			conditions: ProfileFilters{
				ProfileConditions: []string{
					`TestProfileFunc() and IsBool(true)`,
				},
			},
			wantErrorWith: `undefined function "IsBool"`,
			factoryOptions: []FactoryOption{
				WithProfileFunctions([]ottl.Factory[ottlprofile.TransformContext]{createTestFuncFactory[ottlprofile.TransformContext]("TestProfileFunc")}),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := NewFactoryWithOptions(tt.factoryOptions...).(xprocessor.Factory)
			cfg := factory.CreateDefaultConfig()
			oCfg := cfg.(*Config)
			oCfg.ErrorMode = ottl.IgnoreError
			oCfg.Profiles = tt.conditions

			_, err := factory.CreateProfiles(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
			if tt.wantErrorWith != "" {
				if err == nil {
					t.Errorf("expected error containing '%s', got: <nil>", tt.wantErrorWith)
				}
				assert.Contains(t, err.Error(), tt.wantErrorWith)
				return
			}
			require.NoError(t, err)
		})
	}
}
