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

package transformprocessor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configtest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/traces"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/logs"
)

func TestFactory_Type(t *testing.T) {
	factory := NewFactory()
	assert.Equal(t, factory.Type(), config.Type(typeStr))
}

func TestFactory_CreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.Equal(t, cfg, &Config{
		ProcessorSettings: config.NewProcessorSettings(config.NewComponentID(typeStr)),
		Traces: SignalConfig{
			Queries: []string{},

			functions: traces.DefaultFunctions(),
		},
		Metrics: SignalConfig{
			Queries: []string{},

			functions: traces.DefaultFunctions(),
		},
		Logs: SignalConfig{
			Queries: []string{},

			functions: logs.DefaultFunctions(),
		},
	})
	assert.NoError(t, configtest.CheckConfigStruct(cfg))
}

func TestFactoryCreateProcessor_Empty(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	err := cfg.Validate()
	assert.NoError(t, err)
}

func TestFactoryCreateTracesProcessor_InvalidActions(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	oCfg := cfg.(*Config)
	oCfg.Traces.Queries = []string{`set(123`}
	ap, err := factory.CreateTracesProcessor(context.Background(), componenttest.NewNopProcessorCreateSettings(), cfg, consumertest.NewNop())
	assert.Error(t, err)
	assert.Nil(t, ap)
}

func TestFactoryCreateTracesProcessor(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	oCfg := cfg.(*Config)
	oCfg.Traces.Queries = []string{`set(attributes["test"], "pass") where name == "operationA"`}

	tp, err := factory.CreateTracesProcessor(context.Background(), componenttest.NewNopProcessorCreateSettings(), cfg, consumertest.NewNop())
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
	assert.Equal(t, "pass", val.StringVal())
}

func TestFactoryCreateMetricsProcessor_InvalidActions(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	oCfg := cfg.(*Config)
	oCfg.Metrics.Queries = []string{`set(123`}
	ap, err := factory.CreateMetricsProcessor(context.Background(), componenttest.NewNopProcessorCreateSettings(), cfg, consumertest.NewNop())
	assert.Error(t, err)
	assert.Nil(t, ap)
}

func TestFactoryCreateMetricsProcessor(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	oCfg := cfg.(*Config)
	oCfg.Metrics.Queries = []string{`set(attributes["test"], "pass") where metric.name == "operationA"`}

	metricsProcessor, err := factory.CreateMetricsProcessor(context.Background(), componenttest.NewNopProcessorCreateSettings(), cfg, consumertest.NewNop())
	assert.NotNil(t, metricsProcessor)
	assert.NoError(t, err)

	metrics := pmetric.NewMetrics()
	metric := metrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	metric.SetName("operationA")
	metric.SetDataType(pmetric.MetricDataTypeSum)

	_, ok := metric.Sum().DataPoints().AppendEmpty().Attributes().Get("test")
	assert.False(t, ok)

	err = metricsProcessor.ConsumeMetrics(context.Background(), metrics)
	assert.NoError(t, err)

	val, ok := metric.Sum().DataPoints().At(0).Attributes().Get("test")
	assert.True(t, ok)
	assert.Equal(t, "pass", val.StringVal())
}

func TestFactoryCreateLogsProcessor(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	oCfg := cfg.(*Config)
	oCfg.Logs.Queries = []string{`set(attributes["test"], "pass") where body == "operationA"`}

	lp, err := factory.CreateLogsProcessor(context.Background(), componenttest.NewNopProcessorCreateSettings(), cfg, consumertest.NewNop())
	assert.NotNil(t, lp)
	assert.NoError(t, err)

	ld := plog.NewLogs()
	log := ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()
	log.Body().SetStringVal("operationA")

	_, ok := log.Attributes().Get("test")
	assert.False(t, ok)

	err = lp.ConsumeLogs(context.Background(), ld)
	assert.NoError(t, err)

	val, ok := log.Attributes().Get("test")
	assert.True(t, ok)
	assert.Equal(t, "pass", val.StringVal())
}

func TestFactoryCreateLogsProcessor_InvalidActions(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	oCfg := cfg.(*Config)
	oCfg.Logs.Queries = []string{`set(123`}
	ap, err := factory.CreateLogsProcessor(context.Background(), componenttest.NewNopProcessorCreateSettings(), cfg, consumertest.NewNop())
	assert.Error(t, err)
	assert.Nil(t, ap)
}
