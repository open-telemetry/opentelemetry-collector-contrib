// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package traces // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/traces"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"
)

type parsedContextStatements struct {
	common.TracesConsumer
	sharedCache bool
}

type Processor struct {
	contexts []parsedContextStatements
	logger   *zap.Logger
}

func NewProcessor(contextStatements []common.ContextStatements, errorMode ottl.ErrorMode, settings component.TelemetrySettings) (*Processor, error) {
	pc, err := common.NewTraceParserCollection(settings, common.WithSpanParser(SpanFunctions()), common.WithSpanEventParser(SpanEventFunctions()), common.WithTraceErrorMode(errorMode))
	if err != nil {
		return nil, err
	}

	contexts := make([]parsedContextStatements, len(contextStatements))
	var errors error
	for i, cs := range contextStatements {
		context, err := pc.ParseContextStatements(cs)
		if err != nil {
			errors = multierr.Append(errors, err)
		}
		contexts[i] = parsedContextStatements{context, cs.SharedCache}
	}

	if errors != nil {
		return nil, errors
	}

	return &Processor{
		contexts: contexts,
		logger:   settings.Logger,
	}, nil
}

func (p *Processor) ProcessTraces(ctx context.Context, td ptrace.Traces) (ptrace.Traces, error) {
	sharedContextCache := make(map[common.ContextID]*pcommon.Map, len(p.contexts))
	for _, c := range p.contexts {
		cache := common.LoadContextCache(sharedContextCache, c.Context(), c.sharedCache)
		err := c.ConsumeTraces(ctx, td, cache)
		if err != nil {
			p.logger.Error("failed processing traces", zap.Error(err))
			return td, err
		}
	}
	return td, nil
}
