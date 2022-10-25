// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package common // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"
import (
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottldatapoints"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllogs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlresource"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlscope"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottltraces"
)

type Context interface {
	IsContext() bool
}

type TracesContext interface {
	ProcessTraces(td ptrace.Traces)
}

type MetricsContext interface {
	ProcessMetrics(td pmetric.Metrics)
}

type LogsContext interface {
	ProcessLogs(td plog.Logs)
}

var _ Context = &ResourceStatements{}

type ResourceStatements struct {
	Statements []*ottl.Statement[ottlresource.TransformContext]
}

func (r *ResourceStatements) IsContext() bool {
	return true
}

func (r *ResourceStatements) ProcessTraces(td ptrace.Traces) {
	for i := 0; i < td.ResourceSpans().Len(); i++ {
		rspans := td.ResourceSpans().At(i)
		ctx := ottlresource.NewTransformContext(rspans.Resource())
		for _, statement := range r.Statements {
			statement.Execute(ctx)
		}
	}
}

func (r *ResourceStatements) ProcessMetrics(td pmetric.Metrics) {
	for i := 0; i < td.ResourceMetrics().Len(); i++ {
		rmetrics := td.ResourceMetrics().At(i)
		ctx := ottlresource.NewTransformContext(rmetrics.Resource())
		for _, statement := range r.Statements {
			statement.Execute(ctx)
		}
	}
}

func (r *ResourceStatements) ProcessLogs(td plog.Logs) {
	for i := 0; i < td.ResourceLogs().Len(); i++ {
		rlogs := td.ResourceLogs().At(i)
		ctx := ottlresource.NewTransformContext(rlogs.Resource())
		for _, statement := range r.Statements {
			statement.Execute(ctx)
		}
	}
}

var _ Context = &ScopeStatements{}

type ScopeStatements struct {
	Statements []*ottl.Statement[ottlscope.TransformContext]
}

func (s *ScopeStatements) IsContext() bool {
	return true
}

func (s *ScopeStatements) ProcessTraces(td ptrace.Traces) {
	for i := 0; i < td.ResourceSpans().Len(); i++ {
		rspans := td.ResourceSpans().At(i)
		for j := 0; j < rspans.ScopeSpans().Len(); j++ {
			sspans := rspans.ScopeSpans().At(j)
			ctx := ottlscope.NewTransformContext(sspans.Scope(), rspans.Resource())
			for _, statement := range s.Statements {
				statement.Execute(ctx)
			}
		}
	}
}

func (s *ScopeStatements) ProcessMetrics(td pmetric.Metrics) {
	for i := 0; i < td.ResourceMetrics().Len(); i++ {
		rmetrics := td.ResourceMetrics().At(i)
		for j := 0; j < rmetrics.ScopeMetrics().Len(); j++ {
			smetrics := rmetrics.ScopeMetrics().At(j)
			ctx := ottlscope.NewTransformContext(smetrics.Scope(), rmetrics.Resource())
			for _, statement := range s.Statements {
				statement.Execute(ctx)
			}
		}
	}
}

func (s *ScopeStatements) ProcessLogs(td plog.Logs) {
	for i := 0; i < td.ResourceLogs().Len(); i++ {
		rlogs := td.ResourceLogs().At(i)
		for j := 0; j < rlogs.ScopeLogs().Len(); j++ {
			slogs := rlogs.ScopeLogs().At(j)
			ctx := ottlscope.NewTransformContext(slogs.Scope(), rlogs.Resource())
			for _, statement := range s.Statements {
				statement.Execute(ctx)
			}
		}
	}
}

var _ Context = &TraceStatements{}

type TraceStatements struct {
	statements []*ottl.Statement[ottltraces.TransformContext]
}

func (t *TraceStatements) IsContext() bool {
	return true
}

func (t *TraceStatements) ProcessTraces(td ptrace.Traces) {
	for i := 0; i < td.ResourceSpans().Len(); i++ {
		rspans := td.ResourceSpans().At(i)
		for j := 0; j < rspans.ScopeSpans().Len(); j++ {
			sspans := rspans.ScopeSpans().At(j)
			spans := sspans.Spans()
			for k := 0; k < spans.Len(); k++ {
				ctx := ottltraces.NewTransformContext(spans.At(k), sspans.Scope(), rspans.Resource())
				for _, statement := range t.statements {
					statement.Execute(ctx)
				}
			}
		}
	}
}

var _ Context = &LogStatements{}

type LogStatements struct {
	statements []*ottl.Statement[ottllogs.TransformContext]
}

func (l *LogStatements) IsContext() bool {
	return true
}

func (l *LogStatements) ProcessLogs(td plog.Logs) {
	for i := 0; i < td.ResourceLogs().Len(); i++ {
		rlogs := td.ResourceLogs().At(i)
		for j := 0; j < rlogs.ScopeLogs().Len(); j++ {
			slogs := rlogs.ScopeLogs().At(j)
			logs := slogs.LogRecords()
			for k := 0; k < logs.Len(); k++ {
				ctx := ottllogs.NewTransformContext(logs.At(k), slogs.Scope(), rlogs.Resource())
				for _, statement := range l.statements {
					statement.Execute(ctx)
				}
			}
		}
	}
}

var _ Context = &DataPointStatements{}

type DataPointStatements struct {
	statements []*ottl.Statement[ottldatapoints.TransformContext]
}

func (d *DataPointStatements) IsContext() bool {
	return true
}

func (d *DataPointStatements) ProcessMetrics(td pmetric.Metrics) {
	for i := 0; i < td.ResourceMetrics().Len(); i++ {
		rmetrics := td.ResourceMetrics().At(i)
		for j := 0; j < rmetrics.ScopeMetrics().Len(); j++ {
			smetrics := rmetrics.ScopeMetrics().At(j)
			metrics := smetrics.Metrics()
			for k := 0; k < metrics.Len(); k++ {
				metric := metrics.At(k)
				switch metric.Type() {
				case pmetric.MetricTypeSum:
					d.handleNumberDataPoints(metric.Sum().DataPoints(), metrics.At(k), metrics, smetrics.Scope(), rmetrics.Resource())
				case pmetric.MetricTypeGauge:
					d.handleNumberDataPoints(metric.Gauge().DataPoints(), metrics.At(k), metrics, smetrics.Scope(), rmetrics.Resource())
				case pmetric.MetricTypeHistogram:
					d.handleHistogramDataPoints(metric.Histogram().DataPoints(), metrics.At(k), metrics, smetrics.Scope(), rmetrics.Resource())
				case pmetric.MetricTypeExponentialHistogram:
					d.handleExponetialHistogramDataPoints(metric.ExponentialHistogram().DataPoints(), metrics.At(k), metrics, smetrics.Scope(), rmetrics.Resource())
				case pmetric.MetricTypeSummary:
					d.handleSummaryDataPoints(metric.Summary().DataPoints(), metrics.At(k), metrics, smetrics.Scope(), rmetrics.Resource())
				}
			}
		}
	}
}

func (d *DataPointStatements) handleNumberDataPoints(dps pmetric.NumberDataPointSlice, metric pmetric.Metric, metrics pmetric.MetricSlice, is pcommon.InstrumentationScope, resource pcommon.Resource) {
	for i := 0; i < dps.Len(); i++ {
		ctx := ottldatapoints.NewTransformContext(dps.At(i), metric, metrics, is, resource)
		d.callFunctions(ctx)
	}
}

func (d *DataPointStatements) handleHistogramDataPoints(dps pmetric.HistogramDataPointSlice, metric pmetric.Metric, metrics pmetric.MetricSlice, is pcommon.InstrumentationScope, resource pcommon.Resource) {
	for i := 0; i < dps.Len(); i++ {
		ctx := ottldatapoints.NewTransformContext(dps.At(i), metric, metrics, is, resource)
		d.callFunctions(ctx)
	}
}

func (d *DataPointStatements) handleExponetialHistogramDataPoints(dps pmetric.ExponentialHistogramDataPointSlice, metric pmetric.Metric, metrics pmetric.MetricSlice, is pcommon.InstrumentationScope, resource pcommon.Resource) {
	for i := 0; i < dps.Len(); i++ {
		ctx := ottldatapoints.NewTransformContext(dps.At(i), metric, metrics, is, resource)
		d.callFunctions(ctx)
	}
}

func (d *DataPointStatements) handleSummaryDataPoints(dps pmetric.SummaryDataPointSlice, metric pmetric.Metric, metrics pmetric.MetricSlice, is pcommon.InstrumentationScope, resource pcommon.Resource) {
	for i := 0; i < dps.Len(); i++ {
		ctx := ottldatapoints.NewTransformContext(dps.At(i), metric, metrics, is, resource)
		d.callFunctions(ctx)
	}
}

func (d *DataPointStatements) callFunctions(ctx ottldatapoints.TransformContext) {
	for _, statement := range d.statements {
		statement.Execute(ctx)
	}
}

type ParserCollection struct {
	resourceParser   ottl.Parser[ottlresource.TransformContext]
	scopeParser      ottl.Parser[ottlscope.TransformContext]
	traceParser      ottl.Parser[ottltraces.TransformContext]
	dataPointsParser ottl.Parser[ottldatapoints.TransformContext]
	logParser        ottl.Parser[ottllogs.TransformContext]
}

func NewTracesParserCollection(functions map[string]interface{}, settings component.TelemetrySettings) ParserCollection {
	return ParserCollection{
		resourceParser:   ottlresource.NewParser(ResourceFunctions(), settings),
		scopeParser:      ottlscope.NewParser(ScopeFunctions(), settings),
		traceParser:      ottltraces.NewParser(functions, settings),
		dataPointsParser: ottl.Parser[ottldatapoints.TransformContext]{},
		logParser:        ottl.Parser[ottllogs.TransformContext]{},
	}
}

func NewMetricsParserCollection(functions map[string]interface{}, settings component.TelemetrySettings) ParserCollection {
	return ParserCollection{
		resourceParser:   ottlresource.NewParser(ResourceFunctions(), settings),
		scopeParser:      ottlscope.NewParser(ScopeFunctions(), settings),
		traceParser:      ottl.Parser[ottltraces.TransformContext]{},
		dataPointsParser: ottldatapoints.NewParser(functions, settings),
		logParser:        ottl.Parser[ottllogs.TransformContext]{},
	}
}

func NewLogsParserCollection(functions map[string]interface{}, settings component.TelemetrySettings) ParserCollection {
	return ParserCollection{
		resourceParser:   ottlresource.NewParser(ResourceFunctions(), settings),
		scopeParser:      ottlscope.NewParser(ScopeFunctions(), settings),
		traceParser:      ottl.Parser[ottltraces.TransformContext]{},
		dataPointsParser: ottl.Parser[ottldatapoints.TransformContext]{},
		logParser:        ottllogs.NewParser(functions, settings),
	}
}

func (pc ParserCollection) ParseContextStatements(contextStatements []ContextStatements) ([]Context, error) {
	contexts := make([]Context, len(contextStatements))
	var errors error

	for i, s := range contextStatements {
		switch s.Context {
		case Resource:
			statements, err := pc.resourceParser.ParseStatements(s.Statements)
			if err != nil {
				errors = multierr.Append(errors, err)
				continue
			}
			contexts[i] = &ResourceStatements{
				Statements: statements,
			}
		case Scope:
			statements, err := pc.scopeParser.ParseStatements(s.Statements)
			if err != nil {
				errors = multierr.Append(errors, err)
				continue
			}
			contexts[i] = &ScopeStatements{
				Statements: statements,
			}
		case Trace:
			statements, err := pc.traceParser.ParseStatements(s.Statements)
			if err != nil {
				errors = multierr.Append(errors, err)
				continue
			}
			contexts[i] = &TraceStatements{
				statements: statements,
			}
		case DataPoint:
			statements, err := pc.dataPointsParser.ParseStatements(s.Statements)
			if err != nil {
				errors = multierr.Append(errors, err)
				continue
			}
			contexts[i] = &DataPointStatements{
				statements: statements,
			}
		case Log:
			statements, err := pc.logParser.ParseStatements(s.Statements)
			if err != nil {
				errors = multierr.Append(errors, err)
				continue
			}
			contexts[i] = &LogStatements{
				statements: statements,
			}
		default:
			errors = multierr.Append(errors, fmt.Errorf("context, %v, is not a valid context", s.Context))
		}
	}

	if errors != nil {
		return nil, errors
	}
	return contexts, nil
}
