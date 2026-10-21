// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package partitioningprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/partitioningprocessor"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlotelcol"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlresource"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlscope"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
)

type logsPartitioner interface {
	partitionLogs(ctx context.Context, ld plog.Logs) ([]partitionedLogs, error)
}

type partitionedLogs = partitioned[plog.Logs]

// otelcolLogsPartitioner evaluates expressions against the request-scoped otelcol context.
type otelcolLogsPartitioner struct {
	expressions []*ottl.ValueExpression[*ottlotelcol.TransformContext]
}

func (p *otelcolLogsPartitioner) partitionLogs(ctx context.Context, ld plog.Logs) ([]partitionedLogs, error) {
	if ld.ResourceLogs().Len() == 0 {
		return nil, nil
	}
	values := make([]keyValue, len(p.expressions))
	if err := evaluateStringExpressions(ctx, p.expressions, ottlotelcol.NewTransformContext(), values); err != nil {
		return nil, err
	}
	return []partitionedLogs{{values: values, data: ld}}, nil
}

// resourceLogsPartitioner evaluates expressions at the ResourceLogs level.
type resourceLogsPartitioner struct {
	expressions []*ottl.ValueExpression[*ottlresource.TransformContext]
}

func (p *resourceLogsPartitioner) partitionLogs(ctx context.Context, ld plog.Logs) ([]partitionedLogs, error) {
	g := newGrouper(len(p.expressions))
	ids := make([]int32, 0, ld.ResourceLogs().Len())
	for _, rl := range ld.ResourceLogs().All() {
		id, err := assign(ctx, g, p.expressions, ottlresource.NewTransformContext(rl.Resource(), rl))
		if err != nil {
			return nil, err
		}
		ids = append(ids, int32(id))
	}
	if parts, ok := single(g, ld); ok {
		return parts, nil
	}

	parts := newPartitions(g, plog.NewLogs)
	for i, rl := range ld.ResourceLogs().All() {
		rl.MoveTo(parts[ids[i]].data.ResourceLogs().AppendEmpty())
	}
	return parts, nil
}

// scopeLogsPartitioner evaluates expressions at the ScopeLogs level.
type scopeLogsPartitioner struct {
	expressions []*ottl.ValueExpression[*ottlscope.TransformContext]
}

func (p *scopeLogsPartitioner) partitionLogs(ctx context.Context, ld plog.Logs) ([]partitionedLogs, error) {
	g := newGrouper(len(p.expressions))
	var ids []int32
	for _, rl := range ld.ResourceLogs().All() {
		for _, sl := range rl.ScopeLogs().All() {
			id, err := assign(ctx, g, p.expressions, ottlscope.NewTransformContext(sl.Scope(), rl.Resource(), sl, rl))
			if err != nil {
				return nil, err
			}
			ids = append(ids, int32(id))
		}
	}
	if parts, ok := single(g, ld); ok {
		return parts, nil
	}

	parts := newPartitions(g, plog.NewLogs)
	dests := make([]logsDest, len(parts))
	for _, rl := range ld.ResourceLogs().All() {
		for _, sl := range rl.ScopeLogs().All() {
			id := ids[0]
			ids = ids[1:]
			sl.MoveTo(dests[id].resource(parts[id].data, rl).ScopeLogs().AppendEmpty())
		}
	}
	return parts, nil
}

// logRecordPartitioner evaluates expressions at the LogRecord level.
type logRecordPartitioner struct {
	expressions []*ottl.ValueExpression[*ottllog.TransformContext]
}

func (p *logRecordPartitioner) partitionLogs(ctx context.Context, ld plog.Logs) ([]partitionedLogs, error) {
	g := newGrouper(len(p.expressions))
	ids := make([]int32, 0, ld.LogRecordCount())
	for _, rl := range ld.ResourceLogs().All() {
		for _, sl := range rl.ScopeLogs().All() {
			for _, lr := range sl.LogRecords().All() {
				id, err := assign(ctx, g, p.expressions, ottllog.NewTransformContext(rl, sl, lr))
				if err != nil {
					return nil, err
				}
				ids = append(ids, int32(id))
			}
		}
	}
	if parts, ok := single(g, ld); ok {
		return parts, nil
	}

	parts := newPartitions(g, plog.NewLogs)
	dests := make([]logsDest, len(parts))
	for _, rl := range ld.ResourceLogs().All() {
		for _, sl := range rl.ScopeLogs().All() {
			for _, lr := range sl.LogRecords().All() {
				id := ids[0]
				ids = ids[1:]
				lr.MoveTo(dests[id].scope(parts[id].data, rl, sl).LogRecords().AppendEmpty())
			}
		}
	}
	return parts, nil
}

// logsDest tracks a partition's destination containers for the source
// resource and scope currently being visited. Sources are visited in order
// and never revisited, so only the most recent ones can match.
type logsDest struct {
	srcRL, rl plog.ResourceLogs
	srcSL, sl plog.ScopeLogs
}

func (d *logsDest) resource(data plog.Logs, src plog.ResourceLogs) plog.ResourceLogs {
	if d.srcRL != src {
		d.srcRL = src
		d.rl = data.ResourceLogs().AppendEmpty()
		src.Resource().CopyTo(d.rl.Resource())
		d.rl.SetSchemaUrl(src.SchemaUrl())
	}
	return d.rl
}

func (d *logsDest) scope(data plog.Logs, srcRL plog.ResourceLogs, src plog.ScopeLogs) plog.ScopeLogs {
	if d.srcSL != src {
		d.srcSL = src
		d.sl = d.resource(data, srcRL).ScopeLogs().AppendEmpty()
		src.Scope().CopyTo(d.sl.Scope())
		d.sl.SetSchemaUrl(src.SchemaUrl())
	}
	return d.sl
}

func newLogsPartitioner(expressions []string, settings component.TelemetrySettings) (logsPartitioner, error) {
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

	logParser, err := ottllog.NewParser(
		ottlfuncs.StandardConverters[*ottllog.TransformContext](),
		settings,
		ottllog.EnablePathContextNames(),
	)
	if err != nil {
		return nil, err
	}

	pc, err := ottl.NewParserCollection[logsPartitioner](
		settings,
		ottl.WithParserCollectionContext[*ottlotelcol.TransformContext, logsPartitioner](
			ottlotelcol.ContextName,
			&otelcolParser,
			ottl.WithValueExpressionConverter(func(
				_ *ottl.ParserCollection[logsPartitioner],
				_ ottl.ValueExpressionsGetter,
				parsed []*ottl.ValueExpression[*ottlotelcol.TransformContext],
			) (logsPartitioner, error) {
				return &otelcolLogsPartitioner{expressions: parsed}, nil
			}),
		),
		ottl.WithParserCollectionContext[*ottlresource.TransformContext, logsPartitioner](
			ottlresource.ContextName,
			&resourceParser,
			ottl.WithValueExpressionConverter(func(
				_ *ottl.ParserCollection[logsPartitioner],
				_ ottl.ValueExpressionsGetter,
				parsed []*ottl.ValueExpression[*ottlresource.TransformContext],
			) (logsPartitioner, error) {
				return &resourceLogsPartitioner{expressions: parsed}, nil
			}),
		),
		ottl.WithParserCollectionContext[*ottlscope.TransformContext, logsPartitioner](
			ottlscope.ContextName,
			&scopeParser,
			ottl.WithValueExpressionConverter(func(
				_ *ottl.ParserCollection[logsPartitioner],
				_ ottl.ValueExpressionsGetter,
				parsed []*ottl.ValueExpression[*ottlscope.TransformContext],
			) (logsPartitioner, error) {
				return &scopeLogsPartitioner{expressions: parsed}, nil
			}),
		),
		ottl.WithParserCollectionContext[*ottllog.TransformContext, logsPartitioner](
			ottllog.ContextName,
			&logParser,
			ottl.WithValueExpressionConverter(func(
				_ *ottl.ParserCollection[logsPartitioner],
				_ ottl.ValueExpressionsGetter,
				parsed []*ottl.ValueExpression[*ottllog.TransformContext],
			) (logsPartitioner, error) {
				return &logRecordPartitioner{expressions: parsed}, nil
			}),
		),
	)
	if err != nil {
		return nil, err
	}

	return pc.ParseValueExpressions(ottl.NewValueExpressionsGetter(expressions))
}
