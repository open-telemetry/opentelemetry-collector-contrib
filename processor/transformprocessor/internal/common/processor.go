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
	// isContext dummy method for type safety
	isContext()
}

type TracesContext interface {
	ProcessTraces(td ptrace.Traces) error
}

type MetricsContext interface {
	ProcessMetrics(td pmetric.Metrics) error
}

type LogsContext interface {
	ProcessLogs(td plog.Logs) error
}

var _ Context = &resourceStatements{}
var _ TracesContext = &resourceStatements{}
var _ MetricsContext = &resourceStatements{}
var _ LogsContext = &resourceStatements{}

type resourceStatements struct {
	Statements []*ottl.Statement[ottlresource.TransformContext]
}

func (r *resourceStatements) isContext() {}

func (r *resourceStatements) ProcessTraces(td ptrace.Traces) error {
	for i := 0; i < td.ResourceSpans().Len(); i++ {
		rspans := td.ResourceSpans().At(i)
		ctx := ottlresource.NewTransformContext(rspans.Resource())
		for _, statement := range r.Statements {
			_, _, err := statement.Execute(ctx)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *resourceStatements) ProcessMetrics(td pmetric.Metrics) error {
	for i := 0; i < td.ResourceMetrics().Len(); i++ {
		rmetrics := td.ResourceMetrics().At(i)
		ctx := ottlresource.NewTransformContext(rmetrics.Resource())
		for _, statement := range r.Statements {
			_, _, err := statement.Execute(ctx)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *resourceStatements) ProcessLogs(td plog.Logs) error {
	for i := 0; i < td.ResourceLogs().Len(); i++ {
		rlogs := td.ResourceLogs().At(i)
		ctx := ottlresource.NewTransformContext(rlogs.Resource())
		for _, statement := range r.Statements {
			_, _, err := statement.Execute(ctx)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

var _ Context = &scopeStatements{}
var _ TracesContext = &scopeStatements{}
var _ MetricsContext = &scopeStatements{}
var _ LogsContext = &scopeStatements{}

type scopeStatements struct {
	Statements []*ottl.Statement[ottlscope.TransformContext]
}

func (s *scopeStatements) isContext() {}

func (s *scopeStatements) ProcessTraces(td ptrace.Traces) error {
	for i := 0; i < td.ResourceSpans().Len(); i++ {
		rspans := td.ResourceSpans().At(i)
		for j := 0; j < rspans.ScopeSpans().Len(); j++ {
			sspans := rspans.ScopeSpans().At(j)
			ctx := ottlscope.NewTransformContext(sspans.Scope(), rspans.Resource())
			for _, statement := range s.Statements {
				_, _, err := statement.Execute(ctx)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (s *scopeStatements) ProcessMetrics(td pmetric.Metrics) error {
	for i := 0; i < td.ResourceMetrics().Len(); i++ {
		rmetrics := td.ResourceMetrics().At(i)
		for j := 0; j < rmetrics.ScopeMetrics().Len(); j++ {
			smetrics := rmetrics.ScopeMetrics().At(j)
			ctx := ottlscope.NewTransformContext(smetrics.Scope(), rmetrics.Resource())
			for _, statement := range s.Statements {
				_, _, err := statement.Execute(ctx)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (s *scopeStatements) ProcessLogs(td plog.Logs) error {
	for i := 0; i < td.ResourceLogs().Len(); i++ {
		rlogs := td.ResourceLogs().At(i)
		for j := 0; j < rlogs.ScopeLogs().Len(); j++ {
			slogs := rlogs.ScopeLogs().At(j)
			ctx := ottlscope.NewTransformContext(slogs.Scope(), rlogs.Resource())
			for _, statement := range s.Statements {
				_, _, err := statement.Execute(ctx)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

var _ Context = &traceStatements{}
var _ TracesContext = &traceStatements{}

type traceStatements struct {
	statements []*ottl.Statement[ottltraces.TransformContext]
}

func (t *traceStatements) isContext() {}

func (t *traceStatements) ProcessTraces(td ptrace.Traces) error {
	for i := 0; i < td.ResourceSpans().Len(); i++ {
		rspans := td.ResourceSpans().At(i)
		for j := 0; j < rspans.ScopeSpans().Len(); j++ {
			sspans := rspans.ScopeSpans().At(j)
			spans := sspans.Spans()
			for k := 0; k < spans.Len(); k++ {
				ctx := ottltraces.NewTransformContext(spans.At(k), sspans.Scope(), rspans.Resource())
				for _, statement := range t.statements {
					_, _, err := statement.Execute(ctx)
					if err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

var _ Context = &logStatements{}
var _ LogsContext = &logStatements{}

type logStatements struct {
	statements []*ottl.Statement[ottllogs.TransformContext]
}

func (l *logStatements) isContext() {}

func (l *logStatements) ProcessLogs(td plog.Logs) error {
	for i := 0; i < td.ResourceLogs().Len(); i++ {
		rlogs := td.ResourceLogs().At(i)
		for j := 0; j < rlogs.ScopeLogs().Len(); j++ {
			slogs := rlogs.ScopeLogs().At(j)
			logs := slogs.LogRecords()
			for k := 0; k < logs.Len(); k++ {
				ctx := ottllogs.NewTransformContext(logs.At(k), slogs.Scope(), rlogs.Resource())
				for _, statement := range l.statements {
					_, _, err := statement.Execute(ctx)
					if err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

var _ Context = &dataPointStatements{}
var _ MetricsContext = &dataPointStatements{}

type dataPointStatements struct {
	statements []*ottl.Statement[ottldatapoints.TransformContext]
}

func (d *dataPointStatements) isContext() {}

func (d *dataPointStatements) ProcessMetrics(td pmetric.Metrics) error {
	for i := 0; i < td.ResourceMetrics().Len(); i++ {
		rmetrics := td.ResourceMetrics().At(i)
		for j := 0; j < rmetrics.ScopeMetrics().Len(); j++ {
			smetrics := rmetrics.ScopeMetrics().At(j)
			metrics := smetrics.Metrics()
			for k := 0; k < metrics.Len(); k++ {
				metric := metrics.At(k)
				var err error
				switch metric.Type() {
				case pmetric.MetricTypeSum:
					err = d.handleNumberDataPoints(metric.Sum().DataPoints(), metrics.At(k), metrics, smetrics.Scope(), rmetrics.Resource())
				case pmetric.MetricTypeGauge:
					err = d.handleNumberDataPoints(metric.Gauge().DataPoints(), metrics.At(k), metrics, smetrics.Scope(), rmetrics.Resource())
				case pmetric.MetricTypeHistogram:
					err = d.handleHistogramDataPoints(metric.Histogram().DataPoints(), metrics.At(k), metrics, smetrics.Scope(), rmetrics.Resource())
				case pmetric.MetricTypeExponentialHistogram:
					err = d.handleExponetialHistogramDataPoints(metric.ExponentialHistogram().DataPoints(), metrics.At(k), metrics, smetrics.Scope(), rmetrics.Resource())
				case pmetric.MetricTypeSummary:
					err = d.handleSummaryDataPoints(metric.Summary().DataPoints(), metrics.At(k), metrics, smetrics.Scope(), rmetrics.Resource())
				}
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (d *dataPointStatements) handleNumberDataPoints(dps pmetric.NumberDataPointSlice, metric pmetric.Metric, metrics pmetric.MetricSlice, is pcommon.InstrumentationScope, resource pcommon.Resource) error {
	for i := 0; i < dps.Len(); i++ {
		ctx := ottldatapoints.NewTransformContext(dps.At(i), metric, metrics, is, resource)
		err := d.callFunctions(ctx)
		if err != nil {
			return err
		}
	}
	return nil
}

func (d *dataPointStatements) handleHistogramDataPoints(dps pmetric.HistogramDataPointSlice, metric pmetric.Metric, metrics pmetric.MetricSlice, is pcommon.InstrumentationScope, resource pcommon.Resource) error {
	for i := 0; i < dps.Len(); i++ {
		ctx := ottldatapoints.NewTransformContext(dps.At(i), metric, metrics, is, resource)
		err := d.callFunctions(ctx)
		if err != nil {
			return err
		}
	}
	return nil
}

func (d *dataPointStatements) handleExponetialHistogramDataPoints(dps pmetric.ExponentialHistogramDataPointSlice, metric pmetric.Metric, metrics pmetric.MetricSlice, is pcommon.InstrumentationScope, resource pcommon.Resource) error {
	for i := 0; i < dps.Len(); i++ {
		ctx := ottldatapoints.NewTransformContext(dps.At(i), metric, metrics, is, resource)
		err := d.callFunctions(ctx)
		if err != nil {
			return err
		}
	}
	return nil
}

func (d *dataPointStatements) handleSummaryDataPoints(dps pmetric.SummaryDataPointSlice, metric pmetric.Metric, metrics pmetric.MetricSlice, is pcommon.InstrumentationScope, resource pcommon.Resource) error {
	for i := 0; i < dps.Len(); i++ {
		ctx := ottldatapoints.NewTransformContext(dps.At(i), metric, metrics, is, resource)
		err := d.callFunctions(ctx)
		if err != nil {
			return err
		}
	}
	return nil
}

func (d *dataPointStatements) callFunctions(ctx ottldatapoints.TransformContext) error {
	for _, statement := range d.statements {
		_, _, err := statement.Execute(ctx)
		if err != nil {
			return err
		}
	}
	return nil
}

type ParserCollection struct {
	settings         component.TelemetrySettings
	resourceParser   ottl.Parser[ottlresource.TransformContext]
	scopeParser      ottl.Parser[ottlscope.TransformContext]
	traceParser      ottl.Parser[ottltraces.TransformContext]
	dataPointsParser ottl.Parser[ottldatapoints.TransformContext]
	logParser        ottl.Parser[ottllogs.TransformContext]
}

// Option to construct new consumers.
type Option func(*ParserCollection)

func WithTraceParser(functions map[string]interface{}) Option {
	return func(o *ParserCollection) {
		o.traceParser = ottltraces.NewParser(functions, o.settings)
	}
}

func WithLogParser(functions map[string]interface{}) Option {
	return func(o *ParserCollection) {
		o.logParser = ottllogs.NewParser(functions, o.settings)
	}
}

func WithDataPointParser(functions map[string]interface{}) Option {
	return func(o *ParserCollection) {
		o.dataPointsParser = ottldatapoints.NewParser(functions, o.settings)
	}
}

func NewParserCollection(settings component.TelemetrySettings, options ...Option) *ParserCollection {
	pc := &ParserCollection{
		resourceParser: ottlresource.NewParser(ResourceFunctions(), settings),
		scopeParser:    ottlscope.NewParser(ScopeFunctions(), settings),
	}

	for _, op := range options {
		op(pc)
	}

	return pc
}

func (pc *ParserCollection) ParseContextStatements(contextStatements []ContextStatements) ([]Context, error) {
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
			contexts[i] = &resourceStatements{
				Statements: statements,
			}
		case Scope:
			statements, err := pc.scopeParser.ParseStatements(s.Statements)
			if err != nil {
				errors = multierr.Append(errors, err)
				continue
			}
			contexts[i] = &scopeStatements{
				Statements: statements,
			}
		case Trace:
			statements, err := pc.traceParser.ParseStatements(s.Statements)
			if err != nil {
				errors = multierr.Append(errors, err)
				continue
			}
			contexts[i] = &traceStatements{
				statements: statements,
			}
		case DataPoint:
			statements, err := pc.dataPointsParser.ParseStatements(s.Statements)
			if err != nil {
				errors = multierr.Append(errors, err)
				continue
			}
			contexts[i] = &dataPointStatements{
				statements: statements,
			}
		case Log:
			statements, err := pc.logParser.ParseStatements(s.Statements)
			if err != nil {
				errors = multierr.Append(errors, err)
				continue
			}
			contexts[i] = &logStatements{
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
