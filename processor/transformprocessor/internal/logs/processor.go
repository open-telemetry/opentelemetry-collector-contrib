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
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"
)

type parsedContextStatements struct {
	common.LogsConsumer
	sharedCache bool
}

type Processor struct {
	contexts []parsedContextStatements
	logger   *zap.Logger
	flatMode bool
}

func NewProcessor(contextStatements []common.ContextStatements, errorMode ottl.ErrorMode, flatMode bool, settings component.TelemetrySettings) (*Processor, error) {
	pc, err := common.NewLogParserCollection(settings, common.WithLogParser(LogFunctions()), common.WithLogErrorMode(errorMode))
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
		flatMode: flatMode,
	}, nil
}

func (p *Processor) ProcessLogs(ctx context.Context, ld plog.Logs) (plog.Logs, error) {
	if p.flatMode {
		pdatautil.FlattenLogs(ld.ResourceLogs())
		defer pdatautil.GroupByResourceLogs(ld.ResourceLogs())
	}

	sharedContextCache := make(map[common.ContextID]*pcommon.Map, len(p.contexts))
	for _, c := range p.contexts {
		cache := common.LoadContextCache(sharedContextCache, c.Context(), c.sharedCache)
		err := c.ConsumeLogs(ctx, ld, cache)
		if err != nil {
			p.logger.Error("failed processing logs", zap.Error(err))
			return ld, err
		}
	}
	return ld, nil
}
