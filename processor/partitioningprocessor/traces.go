// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package partitioningprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/partitioningprocessor"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlotelcol"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlresource"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlscope"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
)

type tracesPartitioner interface {
	partitionTraces(ctx context.Context, td ptrace.Traces) ([]partitionedTraces, error)
}

type partitionedTraces = partitioned[ptrace.Traces]

// otelcolTracesPartitioner evaluates expressions against the request-scoped otelcol context.
type otelcolTracesPartitioner struct {
	expressions []*ottl.ValueExpression[*ottlotelcol.TransformContext]
}

func (p *otelcolTracesPartitioner) partitionTraces(ctx context.Context, td ptrace.Traces) ([]partitionedTraces, error) {
	if td.ResourceSpans().Len() == 0 {
		return nil, nil
	}
	values := make([]keyValue, len(p.expressions))
	if err := evaluateStringExpressions(ctx, p.expressions, ottlotelcol.NewTransformContext(), values); err != nil {
		return nil, err
	}
	return []partitionedTraces{{values: values, data: td}}, nil
}

// resourceTracesPartitioner evaluates expressions at the ResourceSpans level.
type resourceTracesPartitioner struct {
	expressions []*ottl.ValueExpression[*ottlresource.TransformContext]
}

func (p *resourceTracesPartitioner) partitionTraces(ctx context.Context, td ptrace.Traces) ([]partitionedTraces, error) {
	g := newGrouper(len(p.expressions))
	ids := make([]int32, 0, td.ResourceSpans().Len())
	for _, rs := range td.ResourceSpans().All() {
		id, err := assign(ctx, g, p.expressions, ottlresource.NewTransformContext(rs.Resource(), rs))
		if err != nil {
			return nil, err
		}
		ids = append(ids, int32(id))
	}
	if parts, ok := single(g, td); ok {
		return parts, nil
	}

	parts := newPartitions(g, ptrace.NewTraces)
	for i, rs := range td.ResourceSpans().All() {
		rs.MoveTo(parts[ids[i]].data.ResourceSpans().AppendEmpty())
	}
	return parts, nil
}

// scopeTracesPartitioner evaluates expressions at the ScopeSpans level.
type scopeTracesPartitioner struct {
	expressions []*ottl.ValueExpression[*ottlscope.TransformContext]
}

func (p *scopeTracesPartitioner) partitionTraces(ctx context.Context, td ptrace.Traces) ([]partitionedTraces, error) {
	g := newGrouper(len(p.expressions))
	var ids []int32
	for _, rs := range td.ResourceSpans().All() {
		for _, ss := range rs.ScopeSpans().All() {
			id, err := assign(ctx, g, p.expressions, ottlscope.NewTransformContext(ss.Scope(), rs.Resource(), ss, rs))
			if err != nil {
				return nil, err
			}
			ids = append(ids, int32(id))
		}
	}
	if parts, ok := single(g, td); ok {
		return parts, nil
	}

	parts := newPartitions(g, ptrace.NewTraces)
	dests := make([]tracesDest, len(parts))
	for _, rs := range td.ResourceSpans().All() {
		for _, ss := range rs.ScopeSpans().All() {
			id := ids[0]
			ids = ids[1:]
			ss.MoveTo(dests[id].resource(parts[id].data, rs).ScopeSpans().AppendEmpty())
		}
	}
	return parts, nil
}

// spanTracesPartitioner evaluates expressions at the Span level.
type spanTracesPartitioner struct {
	expressions []*ottl.ValueExpression[*ottlspan.TransformContext]
}

func (p *spanTracesPartitioner) partitionTraces(ctx context.Context, td ptrace.Traces) ([]partitionedTraces, error) {
	g := newGrouper(len(p.expressions))
	ids := make([]int32, 0, td.SpanCount())
	for _, rs := range td.ResourceSpans().All() {
		for _, ss := range rs.ScopeSpans().All() {
			for _, span := range ss.Spans().All() {
				id, err := assign(ctx, g, p.expressions, ottlspan.NewTransformContext(rs, ss, span))
				if err != nil {
					return nil, err
				}
				ids = append(ids, int32(id))
			}
		}
	}
	if parts, ok := single(g, td); ok {
		return parts, nil
	}

	parts := newPartitions(g, ptrace.NewTraces)
	dests := make([]tracesDest, len(parts))
	for _, rs := range td.ResourceSpans().All() {
		for _, ss := range rs.ScopeSpans().All() {
			for _, span := range ss.Spans().All() {
				id := ids[0]
				ids = ids[1:]
				span.MoveTo(dests[id].scope(parts[id].data, rs, ss).Spans().AppendEmpty())
			}
		}
	}
	return parts, nil
}

// tracesDest tracks a partition's destination containers for the source
// resource and scope currently being visited. Sources are visited in order
// and never revisited, so only the most recent ones can match.
type tracesDest struct {
	srcRS, rs ptrace.ResourceSpans
	srcSS, ss ptrace.ScopeSpans
}

func (d *tracesDest) resource(data ptrace.Traces, src ptrace.ResourceSpans) ptrace.ResourceSpans {
	if d.srcRS != src {
		d.srcRS = src
		d.rs = data.ResourceSpans().AppendEmpty()
		src.Resource().CopyTo(d.rs.Resource())
		d.rs.SetSchemaUrl(src.SchemaUrl())
	}
	return d.rs
}

func (d *tracesDest) scope(data ptrace.Traces, srcRS ptrace.ResourceSpans, src ptrace.ScopeSpans) ptrace.ScopeSpans {
	if d.srcSS != src {
		d.srcSS = src
		d.ss = d.resource(data, srcRS).ScopeSpans().AppendEmpty()
		src.Scope().CopyTo(d.ss.Scope())
		d.ss.SetSchemaUrl(src.SchemaUrl())
	}
	return d.ss
}

func newTracesPartitioner(expressions []string, settings component.TelemetrySettings) (tracesPartitioner, error) {
	otelcolParser, err := ottlotelcol.NewParser(
		ottlfuncs.StandardConverters[*ottlotelcol.TransformContext](),
		settings,
		ottlotelcol.EnablePathContextNames(),
	)
	if err != nil {
		return nil, err
	}

	resourceParser, err := ottlresource.NewParser(
		ottlfuncs.StandardConverters[*ottlresource.TransformContext](),
		settings,
		ottlresource.EnablePathContextNames(),
	)
	if err != nil {
		return nil, err
	}

	scopeParser, err := ottlscope.NewParser(
		ottlfuncs.StandardConverters[*ottlscope.TransformContext](),
		settings,
		ottlscope.EnablePathContextNames(),
	)
	if err != nil {
		return nil, err
	}

	spanParser, err := ottlspan.NewParser(
		ottlfuncs.StandardConverters[*ottlspan.TransformContext](),
		settings,
		ottlspan.EnablePathContextNames(),
	)
	if err != nil {
		return nil, err
	}

	pc, err := ottl.NewParserCollection[tracesPartitioner](
		settings,
		ottl.WithParserCollectionContext[*ottlotelcol.TransformContext, tracesPartitioner](
			ottlotelcol.ContextName,
			&otelcolParser,
			ottl.WithValueExpressionConverter(func(
				_ *ottl.ParserCollection[tracesPartitioner],
				_ ottl.ValueExpressionsGetter,
				parsed []*ottl.ValueExpression[*ottlotelcol.TransformContext],
			) (tracesPartitioner, error) {
				return &otelcolTracesPartitioner{expressions: parsed}, nil
			}),
		),
		ottl.WithParserCollectionContext[*ottlresource.TransformContext, tracesPartitioner](
			ottlresource.ContextName,
			&resourceParser,
			ottl.WithValueExpressionConverter(func(
				_ *ottl.ParserCollection[tracesPartitioner],
				_ ottl.ValueExpressionsGetter,
				parsed []*ottl.ValueExpression[*ottlresource.TransformContext],
			) (tracesPartitioner, error) {
				return &resourceTracesPartitioner{expressions: parsed}, nil
			}),
		),
		ottl.WithParserCollectionContext[*ottlscope.TransformContext, tracesPartitioner](
			ottlscope.ContextName,
			&scopeParser,
			ottl.WithValueExpressionConverter(func(
				_ *ottl.ParserCollection[tracesPartitioner],
				_ ottl.ValueExpressionsGetter,
				parsed []*ottl.ValueExpression[*ottlscope.TransformContext],
			) (tracesPartitioner, error) {
				return &scopeTracesPartitioner{expressions: parsed}, nil
			}),
		),
		ottl.WithParserCollectionContext[*ottlspan.TransformContext, tracesPartitioner](
			ottlspan.ContextName,
			&spanParser,
			ottl.WithValueExpressionConverter(func(
				_ *ottl.ParserCollection[tracesPartitioner],
				_ ottl.ValueExpressionsGetter,
				parsed []*ottl.ValueExpression[*ottlspan.TransformContext],
			) (tracesPartitioner, error) {
				return &spanTracesPartitioner{expressions: parsed}, nil
			}),
		),
	)
	if err != nil {
		return nil, err
	}

	return pc.ParseValueExpressions(ottl.NewValueExpressionsGetter(expressions))
}
