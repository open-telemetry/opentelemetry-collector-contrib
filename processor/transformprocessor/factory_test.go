// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package transformprocessor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/confmap/xconfmap"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottldatapoint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspanevent"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/metadata"
)

func assertConfigContainsDefaultFunctions(t *testing.T, config Config) {
	t.Helper()
	for _, f := range DefaultLogFunctions() {
		assert.Contains(t, config.logFunctions, f.Name(), "missing log function %v", f.Name())
	}
	for _, f := range DefaultDataPointFunctions() {
		assert.Contains(t, config.dataPointFunctions, f.Name(), "missing data point function %v", f.Name())
	}
	for _, f := range DefaultMetricFunctions() {
		assert.Contains(t, config.metricFunctions, f.Name(), "missing metric function %v", f.Name())
	}
	for _, f := range DefaultSpanFunctions() {
		assert.Contains(t, config.spanFunctions, f.Name(), "missing span function %v", f.Name())
	}
	for _, f := range DefaultSpanEventFunctions() {
		assert.Contains(t, config.spanEventFunctions, f.Name(), "missing span event function %v", f.Name())
	}
}

func TestFactory_Type(t *testing.T) {
	factory := NewFactory()
	assert.Equal(t, factory.Type(), metadata.Type)
}

func TestFactory_CreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.EqualExportedValues(t, &Config{
		ErrorMode:        ottl.PropagateError,
		TraceStatements:  []common.ContextStatements{},
		MetricStatements: []common.ContextStatements{},
		LogStatements:    []common.ContextStatements{},
	}, cfg)
	assertConfigContainsDefaultFunctions(t, *cfg.(*Config))
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestFactoryCreateProcessor_Empty(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	err := xconfmap.Validate(cfg)
	assert.NoError(t, err)
}

func TestFactoryCreateTraces_InvalidActions(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	oCfg := cfg.(*Config)
	oCfg.TraceStatements = []common.ContextStatements{
		{
			Context:    "span",
			Statements: []string{`set(123`},
		},
	}
	ap, err := factory.CreateTraces(context.Background(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	assert.Error(t, err)
	assert.Nil(t, ap)
}

func TestFactoryCreateTraces(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	oCfg := cfg.(*Config)
	oCfg.ErrorMode = ottl.IgnoreError
	oCfg.TraceStatements = []common.ContextStatements{
		{
			Context: "span",
			Statements: []string{
				`set(attributes["test"], "pass") where name == "operationA"`,
				`set(attributes["test error mode"], ParseJSON(1)) where name == "operationA"`,
			},
		},
	}
	tp, err := factory.CreateTraces(context.Background(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	assert.NotNil(t, tp)
	assert.NoError(t, err)

	td := ptrace.NewTraces()
	span := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
	span.SetName("operationA")

	_, ok := span.Attributes().Get("test")
	assert.False(t, ok)

	err = tp.ConsumeTraces(context.Background(), td)
	assert.NoError(t, err)

	val, ok := span.Attributes().Get("test")
	assert.True(t, ok)
	assert.Equal(t, "pass", val.Str())
}

func TestFactoryCreateMetrics_InvalidActions(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	oCfg := cfg.(*Config)
	oCfg.ErrorMode = ottl.IgnoreError
	oCfg.MetricStatements = []common.ContextStatements{
		{
			Context:    "datapoint",
			Statements: []string{`set(123`},
		},
	}
	ap, err := factory.CreateMetrics(context.Background(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	assert.Error(t, err)
	assert.Nil(t, ap)
}

func TestFactoryCreateMetrics(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	oCfg := cfg.(*Config)
	oCfg.ErrorMode = ottl.IgnoreError
	oCfg.MetricStatements = []common.ContextStatements{
		{
			Context: "datapoint",
			Statements: []string{
				`set(attributes["test"], "pass") where metric.name == "operationA"`,
				`set(attributes["test error mode"], ParseJSON(1)) where metric.name == "operationA"`,
			},
		},
	}
	metricsProcessor, err := factory.CreateMetrics(context.Background(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	assert.NotNil(t, metricsProcessor)
	assert.NoError(t, err)

	metrics := pmetric.NewMetrics()
	metric := metrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	metric.SetName("operationA")

	_, ok := metric.SetEmptySum().DataPoints().AppendEmpty().Attributes().Get("test")
	assert.False(t, ok)

	err = metricsProcessor.ConsumeMetrics(context.Background(), metrics)
	assert.NoError(t, err)

	val, ok := metric.Sum().DataPoints().At(0).Attributes().Get("test")
	assert.True(t, ok)
	assert.Equal(t, "pass", val.Str())
}

func TestFactoryCreateLogs(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	oCfg := cfg.(*Config)
	oCfg.ErrorMode = ottl.IgnoreError
	oCfg.LogStatements = []common.ContextStatements{
		{
			Context: "log",
			Statements: []string{
				`set(attributes["test"], "pass") where body == "operationA"`,
				`set(attributes["test error mode"], ParseJSON(1)) where body == "operationA"`,
			},
		},
	}
	lp, err := factory.CreateLogs(context.Background(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	assert.NotNil(t, lp)
	assert.NoError(t, err)

	ld := plog.NewLogs()
	log := ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	log.Body().SetStr("operationA")

	_, ok := log.Attributes().Get("test")
	assert.False(t, ok)

	err = lp.ConsumeLogs(context.Background(), ld)
	assert.NoError(t, err)

	val, ok := log.Attributes().Get("test")
	assert.True(t, ok)
	assert.Equal(t, "pass", val.Str())
}

func TestFactoryCreateLogs_InvalidActions(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	oCfg := cfg.(*Config)
	oCfg.LogStatements = []common.ContextStatements{
		{
			Context:    "log",
			Statements: []string{`set(123`},
		},
	}
	ap, err := factory.CreateLogs(context.Background(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	assert.Error(t, err)
	assert.Nil(t, ap)
}

func TestFactoryCreateLogProcessor(t *testing.T) {
	tests := []struct {
		name       string
		conditions []string
		statements []string
		want       func(plog.Logs)
		createLogs func() plog.Logs
	}{
		{
			name:       "create logs processor and pass log context is passed with a global condition that meets the specified condition",
			conditions: []string{`body == "operationA"`},
			statements: []string{`set(attributes["test"], "pass")`},
			want: func(td plog.Logs) {
				newLog := td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
				newLog.Attributes().PutStr("test", "pass")
			},
			createLogs: func() plog.Logs {
				ld := plog.NewLogs()
				log := ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
				log.Body().SetStr("operationA")
				return ld
			},
		},
		{
			name:       "create logs processor and pass log context is passed with a statement condition that meets the specified condition",
			conditions: []string{},
			statements: []string{`set(attributes["test"], "pass") where body == "operationA"`},
			want: func(td plog.Logs) {
				newLog := td.ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
				newLog.Attributes().PutStr("test", "pass")
			},
			createLogs: func() plog.Logs {
				ld := plog.NewLogs()
				log := ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
				log.Body().SetStr("operationA")
				return ld
			},
		},
		{
			name:       "create logs processor and pass log context is passed with a global condition that fails the specified condition",
			conditions: []string{`body == "operationB"`},
			statements: []string{`set(attributes["test"], "pass")`},
			want:       func(_ plog.Logs) {},
			createLogs: func() plog.Logs {
				ld := plog.NewLogs()
				log := ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
				log.Body().SetStr("operationA")
				return ld
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()
			oCfg := cfg.(*Config)
			oCfg.ErrorMode = ottl.IgnoreError
			oCfg.LogStatements = []common.ContextStatements{
				{
					Context:    "log",
					Conditions: tt.conditions,
					Statements: tt.statements,
				},
			}
			lp, err := factory.CreateLogs(context.Background(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
			assert.NotNil(t, lp)
			assert.NoError(t, err)

			ld := tt.createLogs()

			err = lp.ConsumeLogs(context.Background(), ld)
			assert.NoError(t, err)

			exLd := tt.createLogs()
			tt.want(exLd)

			assert.Equal(t, exLd, ld)
		})
	}
}

func TestFactoryCreateResourceProcessor(t *testing.T) {
	tests := []struct {
		name       string
		conditions []string
		statements []string
		want       func(plog.Logs)
		createLogs func() plog.Logs
	}{
		{
			name:       "create logs processor and pass resource context is passed with a global condition that meets the specified condition",
			conditions: []string{`attributes["test"] == "foo"`},
			statements: []string{`set(attributes["test"], "pass")`},
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).Resource().Attributes().PutStr("test", "pass")
			},
			createLogs: func() plog.Logs {
				ld := plog.NewLogs()
				ld.ResourceLogs().AppendEmpty().Resource().Attributes().PutStr("test", "foo")
				return ld
			},
		},
		{
			name:       "create logs processor and pass resource context is passed with a statement condition that meets the specified condition",
			conditions: []string{},
			statements: []string{`set(attributes["test"], "pass") where attributes["test"] == "foo"`},
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).Resource().Attributes().PutStr("test", "pass")
			},
			createLogs: func() plog.Logs {
				ld := plog.NewLogs()
				ld.ResourceLogs().AppendEmpty().Resource().Attributes().PutStr("test", "foo")
				return ld
			},
		},
		{
			name:       "create logs processor and pass resource context is passed with a global condition that fails the specified condition",
			conditions: []string{`attributes["test"] == "wrong"`},
			statements: []string{`set(attributes["test"], "pass")`},
			want:       func(_ plog.Logs) {},
			createLogs: func() plog.Logs {
				ld := plog.NewLogs()
				ld.ResourceLogs().AppendEmpty().Resource().Attributes().PutStr("test", "foo")
				return ld
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()
			oCfg := cfg.(*Config)
			oCfg.ErrorMode = ottl.IgnoreError
			oCfg.LogStatements = []common.ContextStatements{
				{
					Context:    "resource",
					Conditions: tt.conditions,
					Statements: tt.statements,
				},
			}
			lp, err := factory.CreateLogs(context.Background(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
			assert.NotNil(t, lp)
			assert.NoError(t, err)

			ld := tt.createLogs()

			err = lp.ConsumeLogs(context.Background(), ld)
			assert.NoError(t, err)

			exLd := tt.createLogs()
			tt.want(exLd)

			assert.Equal(t, exLd, ld)
		})
	}
}

func TestFactoryCreateScopeProcessor(t *testing.T) {
	tests := []struct {
		name       string
		conditions []string
		statements []string
		want       func(plog.Logs)
		createLogs func() plog.Logs
	}{
		{
			name:       "create logs processor and pass scope context is passed with a global condition that meets the specified condition",
			conditions: []string{`attributes["test"] == "foo"`},
			statements: []string{`set(attributes["test"], "pass")`},
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).Scope().Attributes().PutStr("test", "pass")
			},
			createLogs: func() plog.Logs {
				ld := plog.NewLogs()
				ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().Scope().Attributes().PutStr("test", "foo")
				return ld
			},
		},
		{
			name:       "create logs processor and pass scope context is passed with a statement condition that meets the specified condition",
			conditions: []string{},
			statements: []string{`set(attributes["test"], "pass") where attributes["test"] == "foo"`},
			want: func(td plog.Logs) {
				td.ResourceLogs().At(0).ScopeLogs().At(0).Scope().Attributes().PutStr("test", "pass")
			},
			createLogs: func() plog.Logs {
				ld := plog.NewLogs()
				ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().Scope().Attributes().PutStr("test", "foo")
				return ld
			},
		},
		{
			name:       "create logs processor and pass scope context is passed with a global condition that fails the specified condition",
			conditions: []string{`attributes["test"] == "wrong"`},
			statements: []string{`set(attributes["test"], "pass")`},
			want:       func(_ plog.Logs) {},
			createLogs: func() plog.Logs {
				ld := plog.NewLogs()
				ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().Scope().Attributes().PutStr("test", "foo")
				return ld
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()
			oCfg := cfg.(*Config)
			oCfg.ErrorMode = ottl.IgnoreError
			oCfg.LogStatements = []common.ContextStatements{
				{
					Context:    "scope",
					Conditions: tt.conditions,
					Statements: tt.statements,
				},
			}
			lp, err := factory.CreateLogs(context.Background(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
			assert.NotNil(t, lp)
			assert.NoError(t, err)

			ld := tt.createLogs()

			err = lp.ConsumeLogs(context.Background(), ld)
			assert.NoError(t, err)

			exLd := tt.createLogs()
			tt.want(exLd)

			assert.Equal(t, exLd, ld)
		})
	}
}

func TestFactoryCreateMetricProcessor(t *testing.T) {
	tests := []struct {
		name          string
		conditions    []string
		statements    []string
		want          func(pmetric.Metrics)
		createMetrics func() pmetric.Metrics
	}{
		{
			name:       "create metrics processor and pass metric context is passed with a global condition that meets the specified condition",
			conditions: []string{`name == "operationA"`},
			statements: []string{`set(description, "Sum")`},
			want: func(td pmetric.Metrics) {
				newMetric := td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)
				newMetric.SetDescription("Sum")
			},
			createMetrics: func() pmetric.Metrics {
				td := pmetric.NewMetrics()
				metric := td.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
				metric.SetName("operationA")
				return td
			},
		},
		{
			name:       "create metrics processor and pass metric context is passed with a statement condition that meets the specified condition",
			conditions: []string{},
			statements: []string{`set(description, "Sum") where name == "operationA"`},
			want: func(td pmetric.Metrics) {
				newMetric := td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)
				newMetric.SetDescription("Sum")
			},
			createMetrics: func() pmetric.Metrics {
				td := pmetric.NewMetrics()
				metric := td.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
				metric.SetName("operationA")
				return td
			},
		},
		{
			name:       "create metrics processor and pass metric context is passed with a global condition that fails the specified condition",
			conditions: []string{`name == "operationA"`},
			statements: []string{`set(description, "Sum")`},
			want:       func(_ pmetric.Metrics) {},
			createMetrics: func() pmetric.Metrics {
				td := pmetric.NewMetrics()
				metric := td.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
				metric.SetName("operationB")
				return td
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()
			oCfg := cfg.(*Config)
			oCfg.ErrorMode = ottl.IgnoreError
			oCfg.MetricStatements = []common.ContextStatements{
				{
					Context:    "metric",
					Conditions: tt.conditions,
					Statements: tt.statements,
				},
			}
			mp, err := factory.CreateMetrics(context.Background(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
			assert.NotNil(t, mp)
			assert.NoError(t, err)

			td := tt.createMetrics()

			err = mp.ConsumeMetrics(context.Background(), td)
			assert.NoError(t, err)

			exTd := tt.createMetrics()
			tt.want(exTd)

			assert.Equal(t, exTd, td)
		})
	}
}

func TestFactoryCreateDataPointProcessor(t *testing.T) {
	tests := []struct {
		name          string
		conditions    []string
		statements    []string
		want          func(pmetric.Metrics)
		createMetrics func() pmetric.Metrics
	}{
		{
			name:       "create metrics processor and pass datapoint context is passed with a global condition that meets the specified condition",
			conditions: []string{`metric.name == "operationA"`},
			statements: []string{`set(attributes["test"], "pass")`},
			want: func(td pmetric.Metrics) {
				newMetric := td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)
				newMetric.SetEmptySum().DataPoints().AppendEmpty().Attributes().PutStr("test", "pass")
			},
			createMetrics: func() pmetric.Metrics {
				td := pmetric.NewMetrics()
				metric := td.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
				metric.SetEmptySum().DataPoints().AppendEmpty()
				metric.SetName("operationA")
				return td
			},
		},
		{
			name:       "create metrics processor and pass datapoint context is passed with a statement condition that meets the specified condition",
			conditions: []string{},
			statements: []string{`set(attributes["test"], "pass") where metric.name == "operationA"`},
			want: func(td pmetric.Metrics) {
				newMetric := td.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)
				newMetric.SetEmptySum().DataPoints().AppendEmpty().Attributes().PutStr("test", "pass")
			},
			createMetrics: func() pmetric.Metrics {
				td := pmetric.NewMetrics()
				metric := td.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
				metric.SetEmptySum().DataPoints().AppendEmpty()
				metric.SetName("operationA")
				return td
			},
		},
		{
			name:       "create metrics processor and pass datapoint context is passed with a global condition that fails the specified condition",
			conditions: []string{`metric.name == "operationB"`},
			statements: []string{`set(attributes["test"], "pass")`},
			want:       func(_ pmetric.Metrics) {},
			createMetrics: func() pmetric.Metrics {
				td := pmetric.NewMetrics()
				metric := td.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
				metric.SetEmptySum().DataPoints().AppendEmpty()
				metric.SetName("operationA")
				return td
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()
			oCfg := cfg.(*Config)
			oCfg.ErrorMode = ottl.IgnoreError
			oCfg.MetricStatements = []common.ContextStatements{
				{
					Context:    "datapoint",
					Conditions: tt.conditions,
					Statements: tt.statements,
				},
			}
			mp, err := factory.CreateMetrics(context.Background(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
			assert.NotNil(t, mp)
			assert.NoError(t, err)

			td := tt.createMetrics()

			err = mp.ConsumeMetrics(context.Background(), td)
			assert.NoError(t, err)

			exTd := tt.createMetrics()
			tt.want(exTd)

			assert.Equal(t, exTd, td)
		})
	}
}

func TestFactoryCreateSpanProcessor(t *testing.T) {
	tests := []struct {
		name         string
		conditions   []string
		statements   []string
		want         func(ptrace.Traces)
		createTraces func() ptrace.Traces
	}{
		{
			name:       "create traces processor and pass span context is passed with a global condition that meets the specified condition",
			conditions: []string{`name == "operationA"`},
			statements: []string{`set(attributes["test"], "pass")`},
			want: func(td ptrace.Traces) {
				newSpan := td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
				newSpan.Attributes().PutStr("test", "pass")
			},
			createTraces: func() ptrace.Traces {
				td := ptrace.NewTraces()
				span := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
				span.SetName("operationA")
				return td
			},
		},
		{
			name:       "create traces processor and pass span context is passed with a statement condition that meets the specified condition",
			conditions: []string{},
			statements: []string{`set(attributes["test"], "pass") where name == "operationA"`},
			want: func(td ptrace.Traces) {
				newSpan := td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0)
				newSpan.Attributes().PutStr("test", "pass")
			},
			createTraces: func() ptrace.Traces {
				td := ptrace.NewTraces()
				span := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
				span.SetName("operationA")
				return td
			},
		},
		{
			name:       "create traces processor and pass span context is passed with a global condition that fails the specified condition",
			conditions: []string{`name == "operationB"`},
			statements: []string{`set(attributes["test"], "pass")`},
			want:       func(_ ptrace.Traces) {},
			createTraces: func() ptrace.Traces {
				td := ptrace.NewTraces()
				td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()
				return td
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()
			oCfg := cfg.(*Config)
			oCfg.ErrorMode = ottl.IgnoreError
			oCfg.TraceStatements = []common.ContextStatements{
				{
					Context:    "span",
					Conditions: tt.conditions,
					Statements: tt.statements,
				},
			}
			mp, err := factory.CreateTraces(context.Background(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
			assert.NotNil(t, mp)
			assert.NoError(t, err)

			td := tt.createTraces()

			err = mp.ConsumeTraces(context.Background(), td)
			assert.NoError(t, err)

			exTd := tt.createTraces()
			tt.want(exTd)

			assert.Equal(t, exTd, td)
		})
	}
}

func TestFactoryCreateSpanEventProcessor(t *testing.T) {
	tests := []struct {
		name         string
		conditions   []string
		statements   []string
		want         func(ptrace.Traces)
		createTraces func() ptrace.Traces
	}{
		{
			name:       "create traces processor and pass spanevent context is passed with a global condition that meets the specified condition",
			conditions: []string{`name == "eventA"`},
			statements: []string{`set(attributes["test"], "pass")`},
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Events().At(0).Attributes().PutStr("test", "pass")
			},
			createTraces: func() ptrace.Traces {
				td := ptrace.NewTraces()
				event := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty().Events().AppendEmpty()
				event.SetName("eventA")
				return td
			},
		},
		{
			name:       "create traces processor and pass spanevent context is passed with a statement condition that meets the specified condition",
			conditions: []string{},
			statements: []string{`set(attributes["test"], "pass") where name == "eventA"`},
			want: func(td ptrace.Traces) {
				td.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Events().At(0).Attributes().PutStr("test", "pass")
			},
			createTraces: func() ptrace.Traces {
				td := ptrace.NewTraces()
				event := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty().Events().AppendEmpty()
				event.SetName("eventA")
				return td
			},
		},
		{
			name:       "create traces processor and pass spanevent context is passed with a global condition that fails the specified condition",
			conditions: []string{`name == "eventB"`},
			statements: []string{`set(attributes["test"], "pass")`},
			want:       func(_ ptrace.Traces) {},
			createTraces: func() ptrace.Traces {
				td := ptrace.NewTraces()
				event := td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty().Events().AppendEmpty()
				event.SetName("eventA")
				return td
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()
			oCfg := cfg.(*Config)
			oCfg.ErrorMode = ottl.IgnoreError
			oCfg.TraceStatements = []common.ContextStatements{
				{
					Context:    "spanevent",
					Conditions: tt.conditions,
					Statements: tt.statements,
				},
			}
			mp, err := factory.CreateTraces(context.Background(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
			assert.NotNil(t, mp)
			assert.NoError(t, err)

			td := tt.createTraces()

			err = mp.ConsumeTraces(context.Background(), td)
			assert.NoError(t, err)

			exTd := tt.createTraces()
			tt.want(exTd)

			assert.Equal(t, exTd, td)
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
		statements     []common.ContextStatements
		factoryOptions []FactoryOption
		wantErrorWith  string
	}

	tests := []testCase{
		{
			name: "with span functions : statement with added span func",
			statements: []common.ContextStatements{
				{
					Context:    common.ContextID("span"),
					Statements: []string{`set(cache["attr"], TestSpanFunc())`},
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
			statements: []common.ContextStatements{
				{
					Context:    common.ContextID("span"),
					Statements: []string{`set(cache["attr"], TestSpanFunc())`},
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
			statements: []common.ContextStatements{
				{
					Context:    common.ContextID("span"),
					Statements: []string{`testSpanFunc()`},
				},
			},
			factoryOptions: []FactoryOption{
				WithSpanFunctions([]ottl.Factory[ottlspan.TransformContext]{createTestFuncFactory[ottlspan.TransformContext]("testSpanFunc")}),
			},
		},
		{
			name: "with span functions : missing default functions",
			statements: []common.ContextStatements{
				{
					Context:    common.ContextID("span"),
					Statements: []string{`set(attributes["test"], "TestSpanFunc()")`},
				},
			},
			wantErrorWith: `undefined function "set"`,
			factoryOptions: []FactoryOption{
				WithSpanFunctions([]ottl.Factory[ottlspan.TransformContext]{createTestFuncFactory[ottlspan.TransformContext]("TestSpanFunc")}),
			},
		},
		{
			name: "with span event functions : statement with added span event func",
			statements: []common.ContextStatements{
				{
					Context:    common.ContextID("spanevent"),
					Statements: []string{`set(cache["attr"], TestSpanEventFunc())`},
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
			statements: []common.ContextStatements{
				{
					Context:    common.ContextID("spanevent"),
					Statements: []string{`set(cache["attr"], TestSpanEventFunc())`},
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
			statements: []common.ContextStatements{
				{
					Context:    common.ContextID("spanevent"),
					Statements: []string{`testSpanEventFunc()`},
				},
			},
			factoryOptions: []FactoryOption{
				WithSpanEventFunctions([]ottl.Factory[ottlspanevent.TransformContext]{createTestFuncFactory[ottlspanevent.TransformContext]("testSpanEventFunc")}),
			},
		},
		{
			name: "with span event functions : missing default functions",
			statements: []common.ContextStatements{
				{
					Context:    common.ContextID("spanevent"),
					Statements: []string{`set(attributes["test"], "TestSpanEventFunc()")`},
				},
			},
			wantErrorWith: `undefined function "set"`,
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
			oCfg.TraceStatements = tt.statements

			_, err := factory.CreateTraces(context.Background(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
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
		statements     []common.ContextStatements
		factoryOptions []FactoryOption
		wantErrorWith  string
	}

	tests := []testCase{
		{
			name: "with log functions : statement with added log func",
			statements: []common.ContextStatements{
				{
					Context:    common.ContextID("log"),
					Statements: []string{`set(cache["attr"], TestLogFunc())`},
				},
			},
			factoryOptions: []FactoryOption{
				WithLogFunctions(DefaultLogFunctions()),
				WithLogFunctions([]ottl.Factory[ottllog.TransformContext]{createTestFuncFactory[ottllog.TransformContext]("TestLogFunc")}),
			},
		},
		{
			name: "with log functions : statement with missing log func",
			statements: []common.ContextStatements{
				{
					Context:    common.ContextID("log"),
					Statements: []string{`set(cache["attr"], TestLogFunc())`},
				},
			},
			wantErrorWith: `undefined function "TestLogFunc"`,
			factoryOptions: []FactoryOption{
				WithLogFunctions(DefaultLogFunctions()),
			},
		},
		{
			name: "with log functions : only custom functions",
			statements: []common.ContextStatements{
				{
					Context:    common.ContextID("log"),
					Statements: []string{`testLogFunc()`},
				},
			},
			factoryOptions: []FactoryOption{
				WithLogFunctions([]ottl.Factory[ottllog.TransformContext]{createTestFuncFactory[ottllog.TransformContext]("testLogFunc")}),
			},
		},
		{
			name: "with log functions : missing default functions",
			statements: []common.ContextStatements{
				{
					Context:    common.ContextID("log"),
					Statements: []string{`set(cache["attr"], TestLogFunc())`},
				},
			},
			wantErrorWith: `undefined function "set"`,
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
			oCfg.LogStatements = tt.statements

			_, err := factory.CreateLogs(context.Background(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
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
		statements     []common.ContextStatements
		factoryOptions []FactoryOption
		wantErrorWith  string
	}

	tests := []testCase{
		{
			name: "with metric functions : statement with added metric func",
			statements: []common.ContextStatements{
				{
					Context:    common.ContextID("metric"),
					Statements: []string{`set(cache["attr"], TestMetricFunc())`},
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
			statements: []common.ContextStatements{
				{
					Context:    common.ContextID("metric"),
					Statements: []string{`set(cache["attr"], TestMetricFunc())`},
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
			statements: []common.ContextStatements{
				{
					Context:    common.ContextID("metric"),
					Statements: []string{`testMetricFunc()`},
				},
			},
			factoryOptions: []FactoryOption{
				WithMetricFunctions([]ottl.Factory[ottlmetric.TransformContext]{createTestFuncFactory[ottlmetric.TransformContext]("testMetricFunc")}),
			},
		},
		{
			name: "with metric functions : missing default functions",
			statements: []common.ContextStatements{
				{
					Context:    common.ContextID("metric"),
					Statements: []string{`set(description, "TestMetricFunc()")`},
				},
			},
			wantErrorWith: `undefined function "set"`,
			factoryOptions: []FactoryOption{
				WithMetricFunctions([]ottl.Factory[ottlmetric.TransformContext]{createTestFuncFactory[ottlmetric.TransformContext]("TestMetricFunc")}),
			},
		},
		{
			name: "with datapoint functions : statement with added datapoint func",
			statements: []common.ContextStatements{
				{
					Context:    common.ContextID("datapoint"),
					Statements: []string{`set(cache["attr"], TestDataPointFunc())`},
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
			statements: []common.ContextStatements{
				{
					Context:    common.ContextID("datapoint"),
					Statements: []string{`set(cache["attr"], TestDataPointFunc())`},
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
			statements: []common.ContextStatements{
				{
					Context:    common.ContextID("datapoint"),
					Statements: []string{`testDataPointFunc()`},
				},
			},
			factoryOptions: []FactoryOption{
				WithDataPointFunctions([]ottl.Factory[ottldatapoint.TransformContext]{createTestFuncFactory[ottldatapoint.TransformContext]("testDataPointFunc")}),
			},
		},
		{
			name: "with datapoint functions : missing default functions",
			statements: []common.ContextStatements{
				{
					Context:    common.ContextID("datapoint"),
					Statements: []string{`set(attributes["test"], "TestDataPointFunc()")`},
				},
			},
			wantErrorWith: `undefined function "set"`,
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
			oCfg.MetricStatements = tt.statements

			_, err := factory.CreateMetrics(context.Background(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
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
