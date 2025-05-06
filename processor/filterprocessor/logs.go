// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filterprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pipeline"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processorhelper"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/expr"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterlog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor/internal/common"
)

type filterLogProcessor struct {
	consumers []common.LogsConsumer
	skipExpr  expr.BoolExpr[ottllog.TransformContext]
	telemetry *filterTelemetry
	logger    *zap.Logger
}

func newFilterLogsProcessor(set processor.Settings, cfg *Config) (*filterLogProcessor, error) {
	flp := &filterLogProcessor{
		logger: set.Logger,
	}

	fpt, err := newFilterTelemetry(set, pipeline.SignalLogs)
	if err != nil {
		return nil, fmt.Errorf("error creating filter processor telemetry: %w", err)
	}
	flp.telemetry = fpt

	if len(cfg.LogConditions) > 0 {
		pc, err := common.NewLogParserCollection(set.TelemetrySettings, common.WithLogParser(filterottl.StandardLogFuncs()))
		if err != nil {
			return nil, err
		}
		var errors error
		for _, cs := range cfg.LogConditions {
			metricConsumer, err := pc.ParseContextConditions(cs)
			errors = multierr.Append(errors, err)
			flp.consumers = append(flp.consumers, metricConsumer)
		}
		if errors != nil {
			return nil, err
		}
	}

	if cfg.Logs.LogConditions != nil {
		skipExpr, errBoolExpr := filterottl.NewBoolExprForLog(cfg.Logs.LogConditions, filterottl.StandardLogFuncs(), cfg.ErrorMode, set.TelemetrySettings)
		if errBoolExpr != nil {
			return nil, errBoolExpr
		}
		flp.skipExpr = skipExpr
		return flp, nil
	}

	cfgMatch := filterconfig.MatchConfig{}
	if cfg.Logs.Include != nil && !cfg.Logs.Include.isEmpty() {
		cfgMatch.Include = cfg.Logs.Include.matchProperties()
	}

	if cfg.Logs.Exclude != nil && !cfg.Logs.Exclude.isEmpty() {
		cfgMatch.Exclude = cfg.Logs.Exclude.matchProperties()
	}

	skipExpr, err := filterlog.NewSkipExpr(&cfgMatch)
	if err != nil {
		return nil, fmt.Errorf("failed to build skip matcher: %w", err)
	}
	flp.skipExpr = skipExpr

	return flp, nil
}

func (flp *filterLogProcessor) processExprs(ctx context.Context, ld plog.Logs) (plog.Logs, error) {
	var errors error
	ld.ResourceLogs().RemoveIf(func(rl plog.ResourceLogs) bool {
		resource := rl.Resource()
		rl.ScopeLogs().RemoveIf(func(sl plog.ScopeLogs) bool {
			scope := sl.Scope()
			lrs := sl.LogRecords()
			lrs.RemoveIf(func(lr plog.LogRecord) bool {
				skip, err := flp.skipExpr.Eval(ctx, ottllog.NewTransformContext(lr, scope, resource, sl, rl))
				if err != nil {
					errors = multierr.Append(errors, err)
					return false
				}
				return skip
			})

			return sl.LogRecords().Len() == 0
		})
		return rl.ScopeLogs().Len() == 0
	})
	return ld, errors
}

func (flp *filterLogProcessor) processConditions(ctx context.Context, ld plog.Logs) (plog.Logs, error) {
	var errors error
	for _, consumer := range flp.consumers {
		err := consumer.ConsumeLogs(ctx, ld)
		if err != nil {
			errors = multierr.Append(errors, err)
		}
	}
	return ld, errors
}

func (flp *filterLogProcessor) processLogs(ctx context.Context, ld plog.Logs) (plog.Logs, error) {
	if flp.skipExpr == nil {
		return ld, nil
	}

	logCountBeforeFilters := ld.LogRecordCount()
	var processedLogs plog.Logs
	var errors error
	if len(flp.consumers) > 0 {
		processedLogs, errors = flp.processConditions(ctx, ld)
	} else {
		processedLogs, errors = flp.processExprs(ctx, ld)
	}

	logCountAfterFilters := processedLogs.LogRecordCount()
	flp.telemetry.record(ctx, int64(logCountBeforeFilters-logCountAfterFilters))

	if errors != nil {
		flp.logger.Error("failed processing logs", zap.Error(errors))
		return processedLogs, errors
	}
	if processedLogs.ResourceLogs().Len() == 0 {
		return processedLogs, processorhelper.ErrSkipProcessingData
	}
	return processedLogs, nil
}
