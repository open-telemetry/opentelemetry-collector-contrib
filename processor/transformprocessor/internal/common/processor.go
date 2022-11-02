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
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspanevent"
	"go.opentelemetry.io/collector/component"
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

var _ Context = &resourceStatements{}
var _ TracesContext = &resourceStatements{}
var _ MetricsContext = &resourceStatements{}
var _ LogsContext = &resourceStatements{}

type resourceStatements []*ottl.Statement[ottlresource.TransformContext]

func (r resourceStatements) isContext() {}

func (r resourceStatements) ProcessTraces(td ptrace.Traces) error {
	for i := 0; i < td.ResourceSpans().Len(); i++ {
		rspans := td.ResourceSpans().At(i)
		ctx := ottlresource.NewTransformContext(rspans.Resource())
		for _, statement := range r {
			_, _, err := statement.Execute(ctx)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (r resourceStatements) ProcessMetrics(td pmetric.Metrics) error {
	for i := 0; i < td.ResourceMetrics().Len(); i++ {
		rmetrics := td.ResourceMetrics().At(i)
		ctx := ottlresource.NewTransformContext(rmetrics.Resource())
		for _, statement := range r {
			_, _, err := statement.Execute(ctx)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (r resourceStatements) ProcessLogs(td plog.Logs) error {
	for i := 0; i < td.ResourceLogs().Len(); i++ {
		rlogs := td.ResourceLogs().At(i)
		ctx := ottlresource.NewTransformContext(rlogs.Resource())
		for _, statement := range r {
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

type scopeStatements []*ottl.Statement[ottlscope.TransformContext]

func (s scopeStatements) isContext() {}

func (s scopeStatements) ProcessTraces(td ptrace.Traces) error {
	for i := 0; i < td.ResourceSpans().Len(); i++ {
		rspans := td.ResourceSpans().At(i)
		for j := 0; j < rspans.ScopeSpans().Len(); j++ {
			sspans := rspans.ScopeSpans().At(j)
			ctx := ottlscope.NewTransformContext(sspans.Scope(), rspans.Resource())
			for _, statement := range s {
				_, _, err := statement.Execute(ctx)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (s scopeStatements) ProcessMetrics(td pmetric.Metrics) error {
	for i := 0; i < td.ResourceMetrics().Len(); i++ {
		rmetrics := td.ResourceMetrics().At(i)
		for j := 0; j < rmetrics.ScopeMetrics().Len(); j++ {
			smetrics := rmetrics.ScopeMetrics().At(j)
			ctx := ottlscope.NewTransformContext(smetrics.Scope(), rmetrics.Resource())
			for _, statement := range s {
				_, _, err := statement.Execute(ctx)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (s scopeStatements) ProcessLogs(td plog.Logs) error {
	for i := 0; i < td.ResourceLogs().Len(); i++ {
		rlogs := td.ResourceLogs().At(i)
		for j := 0; j < rlogs.ScopeLogs().Len(); j++ {
			slogs := rlogs.ScopeLogs().At(j)
			ctx := ottlscope.NewTransformContext(slogs.Scope(), rlogs.Resource())
			for _, statement := range s {
				_, _, err := statement.Execute(ctx)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

type ParserCollection struct {
	settings         component.TelemetrySettings
	resourceParser   ottl.Parser[ottlresource.TransformContext]
	scopeParser      ottl.Parser[ottlscope.TransformContext]
	traceParser      ottl.Parser[ottltraces.TransformContext]
	spanEventParser  ottl.Parser[ottlspanevent.TransformContext]
	metricParser     ottl.Parser[ottlmetric.TransformContext]
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

func WithSpanEventParser(functions map[string]interface{}) Option {
	return func(o *ParserCollection) {
		o.spanEventParser = ottlspanevent.NewParser(functions, o.settings)
	}
}

func WithLogParser(functions map[string]interface{}) Option {
	return func(o *ParserCollection) {
		o.logParser = ottllogs.NewParser(functions, o.settings)
	}
}

func WithMetricParser(functions map[string]interface{}) Option {
	return func(o *ParserCollection) {
		o.metricParser = ottlmetric.NewParser(functions, o.settings)
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
			contexts[i] = resourceStatements(statements)
		case Scope:
			statements, err := pc.scopeParser.ParseStatements(s.Statements)
			if err != nil {
				errors = multierr.Append(errors, err)
				continue
			}
			contexts[i] = scopeStatements(statements)
		case Trace:
			statements, err := pc.traceParser.ParseStatements(s.Statements)
			if err != nil {
				errors = multierr.Append(errors, err)
				continue
			}
			contexts[i] = traceStatements(statements)
		case SpanEvent:
			statements, err := pc.spanEventParser.ParseStatements(s.Statements)
			if err != nil {
				errors = multierr.Append(errors, err)
				continue
			}
			contexts[i] = spanEventStatements(statements)
		case Metric:
			statements, err := pc.metricParser.ParseStatements(s.Statements)
			if err != nil {
				errors = multierr.Append(errors, err)
				continue
			}
			contexts[i] = metricStatements(statements)
		case DataPoint:
			statements, err := pc.dataPointsParser.ParseStatements(s.Statements)
			if err != nil {
				errors = multierr.Append(errors, err)
				continue
			}
			contexts[i] = dataPointStatements(statements)
		case Log:
			statements, err := pc.logParser.ParseStatements(s.Statements)
			if err != nil {
				errors = multierr.Append(errors, err)
				continue
			}
			contexts[i] = logStatements(statements)

		default:
			errors = multierr.Append(errors, fmt.Errorf("context, %v, is not a valid context", s.Context))
		}
	}

	if errors != nil {
		return nil, errors
	}
	return contexts, nil
}
