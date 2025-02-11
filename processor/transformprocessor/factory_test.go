// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package transformprocessor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/metadata"
)

func TestFactory_Type(t *testing.T) {
	factory := NewFactory()
	assert.Equal(t, factory.Type(), metadata.Type)
}

func TestFactory_CreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.Equal(t, &Config{
		ErrorMode:        ottl.PropagateError,
		TraceStatements:  []common.ContextStatements{},
		MetricStatements: []common.ContextStatements{},
		LogStatements:    []common.ContextStatements{},
	}, cfg)
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestFactoryCreateProcessor_Empty(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	err := component.ValidateConfig(cfg)
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
	ap, err := factory.CreateTraces(context.Background(), processortest.NewNopSettings(), cfg, consumertest.NewNop())
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
	tp, err := factory.CreateTraces(context.Background(), processortest.NewNopSettings(), cfg, consumertest.NewNop())
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
	ap, err := factory.CreateMetrics(context.Background(), processortest.NewNopSettings(), cfg, consumertest.NewNop())
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
	metricsProcessor, err := factory.CreateMetrics(context.Background(), processortest.NewNopSettings(), cfg, consumertest.NewNop())
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
	lp, err := factory.CreateLogs(context.Background(), processortest.NewNopSettings(), cfg, consumertest.NewNop())
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
	ap, err := factory.CreateLogs(context.Background(), processortest.NewNopSettings(), cfg, consumertest.NewNop())
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
			lp, err := factory.CreateLogs(context.Background(), processortest.NewNopSettings(), cfg, consumertest.NewNop())
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
			lp, err := factory.CreateLogs(context.Background(), processortest.NewNopSettings(), cfg, consumertest.NewNop())
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
			lp, err := factory.CreateLogs(context.Background(), processortest.NewNopSettings(), cfg, consumertest.NewNop())
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
			mp, err := factory.CreateMetrics(context.Background(), processortest.NewNopSettings(), cfg, consumertest.NewNop())
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
			mp, err := factory.CreateMetrics(context.Background(), processortest.NewNopSettings(), cfg, consumertest.NewNop())
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
			mp, err := factory.CreateTraces(context.Background(), processortest.NewNopSettings(), cfg, consumertest.NewNop())
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
			mp, err := factory.CreateTraces(context.Background(), processortest.NewNopSettings(), cfg, consumertest.NewNop())
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
