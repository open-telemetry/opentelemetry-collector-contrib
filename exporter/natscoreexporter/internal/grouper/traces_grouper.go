// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package grouper // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/natscoreexporter/internal/grouper"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
)

type tracesGrouper struct {
	valueExpression *ottl.ValueExpression[ottlspan.TransformContext]
}

func (g *tracesGrouper) Group(ctx context.Context, srcTraces ptrace.Traces) ([]Group[ptrace.Traces], error) {
	var errs error

	type destContext struct {
		traces           ptrace.Traces
		srcResourceSpans ptrace.ResourceSpans
		srcScopeSpans    ptrace.ScopeSpans
	}
	destBySubject := make(map[string]*destContext)

	for _, srcResourceSpans := range srcTraces.ResourceSpans().All() {
		var (
			srcResource       = srcResourceSpans.Resource()
			srcResourceSchema = srcResourceSpans.SchemaUrl()
		)

		for _, srcScopeSpans := range srcResourceSpans.ScopeSpans().All() {
			var (
				srcScope       = srcScopeSpans.Scope()
				srcScopeSchema = srcScopeSpans.SchemaUrl()
			)

			for _, srcSpan := range srcScopeSpans.Spans().All() {
				transformContext := ottlspan.NewTransformContext(
					srcSpan,
					srcScope,
					srcResource,
					srcScopeSpans,
					srcResourceSpans,
				)
				subjectAsAny, err := g.valueExpression.Eval(ctx, transformContext)
				if err != nil {
					errs = multierr.Append(errs, err)
					continue
				}
				subject := subjectAsAny.(string)

				dest, ok := destBySubject[subject]
				if !ok {
					dest = &destContext{traces: ptrace.NewTraces()}
					destBySubject[subject] = dest
				}
				destTraces := dest.traces

				destResourceSpansSlice := destTraces.ResourceSpans()
				if dest.srcResourceSpans != srcResourceSpans {
					dest.srcResourceSpans = srcResourceSpans

					destResourceSpans := destResourceSpansSlice.AppendEmpty()
					srcResource.CopyTo(destResourceSpans.Resource())
					destResourceSpans.SetSchemaUrl(srcResourceSchema)
				}
				destResourceSpans := destResourceSpansSlice.At(destResourceSpansSlice.Len() - 1)

				destScopeSpansSlice := destResourceSpans.ScopeSpans()
				if dest.srcScopeSpans != srcScopeSpans {
					dest.srcScopeSpans = srcScopeSpans

					destScopeSpans := destScopeSpansSlice.AppendEmpty()
					srcScope.CopyTo(destScopeSpans.Scope())
					destScopeSpans.SetSchemaUrl(srcScopeSchema)
				}
				destScopeSpans := destScopeSpansSlice.At(destScopeSpansSlice.Len() - 1)

				destSpanSlice := destScopeSpans.Spans()
				srcSpan.CopyTo(destSpanSlice.AppendEmpty())
			}
		}
	}

	groups := make([]Group[ptrace.Traces], 0, len(destBySubject))
	for subject, dest := range destBySubject {
		groups = append(groups, Group[ptrace.Traces]{
			Subject: subject,
			Data:    dest.traces,
		})
	}
	return groups, errs
}

var _ Grouper[ptrace.Traces] = &tracesGrouper{}

func NewTracesGrouper(subject string, telemetrySettings component.TelemetrySettings) (Grouper[ptrace.Traces], error) {
	parser, err := ottlspan.NewParser(
		ottlfuncs.StandardConverters[ottlspan.TransformContext](),
		telemetrySettings,
	)
	if err != nil {
		return nil, err
	}

	valueExpression, err := parser.ParseValueExpression(subject)
	if err != nil {
		return nil, err
	}

	return &tracesGrouper{valueExpression: valueExpression}, nil
}

var _ NewGrouperFunc[ptrace.Traces] = NewTracesGrouper
