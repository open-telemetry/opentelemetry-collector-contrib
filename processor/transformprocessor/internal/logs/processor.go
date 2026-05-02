// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logs // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/logs"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/pdatautil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/metadata"
)

type Processor struct {
	contexts []common.LogsConsumer
	logger   *zap.Logger
	tracer   trace.Tracer
	flatMode bool
}

func NewProcessor(contextStatements []common.ContextStatements, errorMode ottl.ErrorMode, flatMode bool, settings component.TelemetrySettings, logFunctions map[string]ottl.Factory[*ottllog.TransformContext]) (*Processor, error) {
	pc, err := common.NewLogParserCollection(settings, common.WithLogParser(logFunctions), common.WithLogErrorMode(errorMode))
	if err != nil {
		return nil, err
	}

	contexts := make([]common.LogsConsumer, len(contextStatements))
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
		flatMode: flatMode,
	}, nil
}

func (p *Processor) ProcessLogs(ctx context.Context, ld plog.Logs) (plog.Logs, error) {
	ctx, span := p.tracer.Start(ctx, "transform.ProcessLogs")
	defer span.End()

	if p.flatMode {
		pdatautil.FlattenLogs(ld.ResourceLogs())
		defer pdatautil.GroupByResourceLogs(ld.ResourceLogs())
	}

	for _, c := range p.contexts {
		ctxChild, contextSpan := p.tracer.Start(ctx, "transform.context",
			trace.WithAttributes(attribute.String("transform.context.type", string(c.Context()))))
		err := c.ConsumeLogs(ctxChild, ld)
		if err != nil {
			contextSpan.RecordError(err)
			contextSpan.End()
			p.logger.Error("failed processing logs", zap.Error(err))
			span.RecordError(err)
			return ld, err
		}
		contextSpan.End()
	}

	if span.IsRecording() {
		span.SetAttributes(
			attribute.Int("transform.context.count", len(p.contexts)),
			attribute.Int("transform.resource_logs.count", ld.ResourceLogs().Len()),
		)
	}

	return ld, nil
}
