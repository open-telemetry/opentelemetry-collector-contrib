// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logs // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/logs"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/pdatautil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"
)

type parsedContextStatements struct {
	common.LogsConsumer
	sharedCache bool
}

type Processor struct {
	contexts     []parsedContextStatements
	logger       *zap.Logger
	flatMode     bool
	sharedCaches map[common.ContextID]*pcommon.Map
}

func NewProcessor(contextStatements []common.ContextStatements, errorMode ottl.ErrorMode, flatMode bool, settings component.TelemetrySettings, logFunctions map[string]ottl.Factory[*ottllog.TransformContext]) (*Processor, error) {
	pc, err := common.NewLogParserCollection(settings, common.WithLogParser(logFunctions), common.WithLogErrorMode(errorMode))
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
			LogsConsumer: context,
			sharedCache:  cs.SharedCache,
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
		flatMode:     flatMode,
		sharedCaches: sharedCaches,
	}, nil
}

func (p *Processor) ProcessLogs(ctx context.Context, ld plog.Logs) (plog.Logs, error) {
	if p.flatMode {
		pdatautil.FlattenLogs(ld.ResourceLogs())
		defer pdatautil.GroupByResourceLogs(ld.ResourceLogs())
	}

	for _, c := range p.contexts {
		cache := common.LoadContextCache(p.sharedCaches, c.Context(), c.sharedCache)
		err := c.ConsumeLogs(ctx, ld, cache)
		if err != nil {
			p.logger.Error("failed processing logs", zap.Error(err))
			return ld, err
		}
	}
	return ld, nil
}
