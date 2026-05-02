// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package traces // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/traces"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspanevent"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/metadata"
)

type Processor struct {
	contexts []common.TracesConsumer
	logger   *zap.Logger
	tracer   trace.Tracer
}

func NewProcessor(contextStatements []common.ContextStatements, errorMode ottl.ErrorMode, settings component.TelemetrySettings, spanFunctions map[string]ottl.Factory[*ottlspan.TransformContext], spanEventFunctions map[string]ottl.Factory[*ottlspanevent.TransformContext]) (*Processor, error) {
	pc, err := common.NewTraceParserCollection(settings, common.WithSpanParser(spanFunctions), common.WithSpanEventParser(spanEventFunctions), common.WithTraceErrorMode(errorMode))
	if err != nil {
		return nil, err
	}

	contexts := make([]common.TracesConsumer, len(contextStatements))
	var errors error
	for i, cs := range contextStatements {
		context, err := pc.ParseContextStatements(cs)
		if err != nil {
			errors = multierr.Append(errors, err)
		}
		contexts[i] = context
	}

	if errors != nil {
		return nil, errors
	}

	return &Processor{
		contexts: contexts,
		logger:   settings.Logger,
		tracer:   metadata.Tracer(settings),
	}, nil
}

func (p *Processor) ProcessTraces(ctx context.Context, td ptrace.Traces) (ptrace.Traces, error) {
	ctx, span := p.tracer.Start(ctx, "transform.ProcessTraces")
	defer span.End()

	for _, c := range p.contexts {
		ctxChild, contextSpan := p.tracer.Start(ctx, "transform.context",
			trace.WithAttributes(attribute.String("transform.context.type", string(c.Context()))))
		err := c.ConsumeTraces(ctxChild, td)
		if err != nil {
			contextSpan.RecordError(err)
			contextSpan.End()
			p.logger.Error("failed processing traces", zap.Error(err))
			span.RecordError(err)
			return td, err
		}
		contextSpan.End()
	}

	if span.IsRecording() {
		span.SetAttributes(
			attribute.Int("transform.context.count", len(p.contexts)),
			attribute.Int("transform.resource_spans.count", td.ResourceSpans().Len()),
		)
	}

	return td, nil
}
