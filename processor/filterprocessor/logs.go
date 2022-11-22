// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package filterprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/processor/processorhelper"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/expr"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterlog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor/internal/common"
)

type filterLogProcessor struct {
	cfg           *Config
	skipExpr      expr.BoolExpr[ottllog.TransformContext]
	logger        *zap.Logger
	logConditions []*ottl.Statement[ottllog.TransformContext]
}

func newFilterLogsProcessor(logger *zap.Logger, cfg *Config) (*filterLogProcessor, error) {
	if cfg.Logs.LogConditions != nil {
		logp := ottllog.NewParser(common.Functions[ottllog.TransformContext](), component.TelemetrySettings{Logger: zap.NewNop()})
		statements, err := logp.ParseStatements(common.PrepareConditionForParsing(cfg.Logs.LogConditions))
		if err != nil {
			return nil, err
		}

		return &filterLogProcessor{
			cfg:           cfg,
			logger:        logger,
			logConditions: statements,
		}, nil
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

	return &filterLogProcessor{
		cfg:      cfg,
		skipExpr: skipExpr,
		logger:   logger,
	}, nil
}

func (flp *filterLogProcessor) processLogs(ctx context.Context, logs plog.Logs) (plog.Logs, error) {
	filteringLogs := flp.logConditions != nil

	if filteringLogs {
		var errors error
		logs.ResourceLogs().RemoveIf(func(rlogs plog.ResourceLogs) bool {
			rlogs.ScopeLogs().RemoveIf(func(slogs plog.ScopeLogs) bool {
				slogs.LogRecords().RemoveIf(func(log plog.LogRecord) bool {
					tCtx := ottllog.NewTransformContext(log, slogs.Scope(), rlogs.Resource())
					metCondition, err := common.CheckConditions(ctx, tCtx, flp.logConditions)
					if err != nil {
						errors = multierr.Append(errors, err)
					}
					return metCondition
				})
				return slogs.LogRecords().Len() == 0
			})
			return rlogs.ScopeLogs().Len() == 0
		})

		if errors != nil {
			return logs, errors
		}
		if logs.ResourceLogs().Len() == 0 {
			return logs, processorhelper.ErrSkipProcessingData
		}
		return logs, nil
	}

	rLogs := logs.ResourceLogs()

	var errors error
	rLogs.RemoveIf(func(rl plog.ResourceLogs) bool {
		resource := rl.Resource()
		rl.ScopeLogs().RemoveIf(func(sl plog.ScopeLogs) bool {
			scope := sl.Scope()
			lrs := sl.LogRecords()

			lrs.RemoveIf(func(lr plog.LogRecord) bool {
				skip, err := flp.skipExpr.Eval(ctx, ottllog.NewTransformContext(lr, scope, resource))
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

	if errors != nil {
		return logs, errors
	}
	if rLogs.Len() == 0 {
		return logs, processorhelper.ErrSkipProcessingData
	}

	return logs, nil
}
