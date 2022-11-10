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
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlresource"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlscope"
)

type shouldRemove bool

func executeStatements[K any](ctx context.Context, tCtx K, statements []*ottl.Statement[K]) (shouldRemove, error) {
	for _, statement := range statements {
		value, _, err := statement.Execute(ctx, tCtx)
		if err != nil {
			return false, err
		}
		if remove, ok := value.(shouldRemove); ok && bool(remove) {
			return true, nil
		}
	}
	return false, nil
}

var _ consumer.Traces = &resourceStatements{}
var _ consumer.Metrics = &resourceStatements{}
var _ consumer.Logs = &resourceStatements{}
var _ baseContext = &resourceStatements{}

type resourceStatements []*ottl.Statement[ottlresource.TransformContext]

func (r resourceStatements) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{
		MutatesData: true,
	}
}

func (r resourceStatements) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	var errors error
	td.ResourceSpans().RemoveIf(func(rspans ptrace.ResourceSpans) bool {
		tCtx := ottlresource.NewTransformContext(rspans.Resource())
		remove, err := executeStatements(ctx, tCtx, r)
		if err != nil {
			errors = multierr.Append(errors, err)
			return false
		}
		return bool(remove)
	})
	return errors
}

func (r resourceStatements) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	var errors error
	md.ResourceMetrics().RemoveIf(func(rmetrics pmetric.ResourceMetrics) bool {
		tCtx := ottlresource.NewTransformContext(rmetrics.Resource())
		remove, err := executeStatements(ctx, tCtx, r)
		if err != nil {
			errors = multierr.Append(errors, err)
			return false
		}
		return bool(remove)
	})
	return errors
}

func (r resourceStatements) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	var errors error
	ld.ResourceLogs().RemoveIf(func(rlogs plog.ResourceLogs) bool {
		tCtx := ottlresource.NewTransformContext(rlogs.Resource())
		remove, err := executeStatements(ctx, tCtx, r)
		if err != nil {
			errors = multierr.Append(errors, err)
			return false
		}
		return bool(remove)
	})
	return errors
}

var _ consumer.Traces = &scopeStatements{}
var _ consumer.Metrics = &scopeStatements{}
var _ consumer.Logs = &scopeStatements{}
var _ baseContext = &scopeStatements{}

type scopeStatements []*ottl.Statement[ottlscope.TransformContext]

func (s scopeStatements) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{
		MutatesData: true,
	}
}

func (s scopeStatements) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	var errors error
	td.ResourceSpans().RemoveIf(func(rspans ptrace.ResourceSpans) bool {
		rspans.ScopeSpans().RemoveIf(func(sspans ptrace.ScopeSpans) bool {
			tCtx := ottlscope.NewTransformContext(sspans.Scope(), rspans.Resource())
			remove, err := executeStatements(ctx, tCtx, s)
			if err != nil {
				errors = multierr.Append(errors, err)
				return false
			}
			return bool(remove)
		})
		return rspans.ScopeSpans().Len() == 0
	})
	return errors
}

func (s scopeStatements) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	var errors error
	md.ResourceMetrics().RemoveIf(func(rmetrics pmetric.ResourceMetrics) bool {
		rmetrics.ScopeMetrics().RemoveIf(func(smetrics pmetric.ScopeMetrics) bool {
			tCtx := ottlscope.NewTransformContext(smetrics.Scope(), rmetrics.Resource())
			remove, err := executeStatements(ctx, tCtx, s)
			if err != nil {
				errors = multierr.Append(errors, err)
				return false
			}
			return bool(remove)
		})
		return rmetrics.ScopeMetrics().Len() == 0
	})
	return errors
}

func (s scopeStatements) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	var errors error
	ld.ResourceLogs().RemoveIf(func(rlogs plog.ResourceLogs) bool {
		rlogs.ScopeLogs().RemoveIf(func(slogs plog.ScopeLogs) bool {
			tCtx := ottlscope.NewTransformContext(slogs.Scope(), rlogs.Resource())
			remove, err := executeStatements(ctx, tCtx, s)
			if err != nil {
				errors = multierr.Append(errors, err)
				return false
			}
			return bool(remove)
		})
		return rlogs.ScopeLogs().Len() == 0
	})
	return errors
}

type parserCollection struct {
	settings       component.TelemetrySettings
	resourceParser ottl.Parser[ottlresource.TransformContext]
	scopeParser    ottl.Parser[ottlscope.TransformContext]
}

type baseContext interface {
	consumer.Traces
	consumer.Metrics
	consumer.Logs
}

func (pc parserCollection) parseCommonContextStatements(contextStatement ContextStatements) (baseContext, error) {
	switch contextStatement.Context {
	case Resource:
		statements, err := pc.resourceParser.ParseStatements(contextStatement.Statements)
		if err != nil {
			return nil, err
		}
		return resourceStatements(statements), nil
	case Scope:
		statements, err := pc.scopeParser.ParseStatements(contextStatement.Statements)
		if err != nil {
			return nil, err
		}
		return scopeStatements(statements), nil
	default:
		return nil, fmt.Errorf("unknown context %v", contextStatement.Context)
	}
}
