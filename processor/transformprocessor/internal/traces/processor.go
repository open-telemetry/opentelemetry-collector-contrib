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
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspanevent"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"
)

type parsedContextStatements struct {
	common.TracesConsumer
	sharedCache bool
}

type Processor struct {
	contexts     []parsedContextStatements
	logger       *zap.Logger
	sharedCaches map[common.ContextID]*pcommon.Map
}

func NewProcessor(contextStatements []common.ContextStatements, errorMode ottl.ErrorMode, settings component.TelemetrySettings, spanFunctions map[string]ottl.Factory[*ottlspan.TransformContext], spanEventFunctions map[string]ottl.Factory[*ottlspanevent.TransformContext]) (*Processor, error) {
	pc, err := common.NewTraceParserCollection(settings, common.WithSpanParser(spanFunctions), common.WithSpanEventParser(spanEventFunctions), common.WithTraceErrorMode(errorMode))
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
		contexts[i] = parsedContextStatements{
			TracesConsumer: context,
			sharedCache:    cs.SharedCache,
		}
	}

	if errors != nil {
		return nil, errors
	}

	var sharedCaches map[common.ContextID]*pcommon.Map
	for _, c := range contexts {
		if c.sharedCache {
			if sharedCaches == nil {
				sharedCaches = map[common.ContextID]*pcommon.Map{}
			}
			m := pcommon.NewMap()
			sharedCaches[c.Context()] = &m
		}
	}

	return &Processor{
		contexts:     contexts,
		logger:       settings.Logger,
		sharedCaches: sharedCaches,
	}, nil
}

func (p *Processor) ProcessTraces(ctx context.Context, td ptrace.Traces) (ptrace.Traces, error) {
	for _, c := range p.contexts {
		cache := common.LoadContextCache(p.sharedCaches, c.Context(), c.sharedCache)
		err := c.ConsumeTraces(ctx, td, cache)
		if err != nil {
			p.logger.Error("failed processing traces", zap.Error(err))
			return td, err
		}
	}
	return td, nil
}
