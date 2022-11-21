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

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/processor/filterlog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor/internal/common"
)

type filterLogProcessor struct {
	cfg            *Config
	excludeMatcher filterlog.Matcher
	includeMatcher filterlog.Matcher
	logger         *zap.Logger
	logConditions  []*ottl.Statement[ottllog.TransformContext]
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

	var includeMatcher filterlog.Matcher
	var excludeMatcher filterlog.Matcher

	if cfg.Logs.Include != nil && !cfg.Logs.Include.isEmpty() {
		var err error
		includeMatcher, err = filterlog.NewMatcher(cfg.Logs.Include.matchProperties())
		if err != nil {
			return nil, fmt.Errorf("failed to build include matcher: %w", err)
		}
	}

	if cfg.Logs.Exclude != nil && !cfg.Logs.Exclude.isEmpty() {
		var err error
		excludeMatcher, err = filterlog.NewMatcher(cfg.Logs.Exclude.matchProperties())
		if err != nil {
			return nil, fmt.Errorf("failed to build exclude matcher: %w", err)
		}
	}

	return &filterLogProcessor{
		cfg:            cfg,
		excludeMatcher: excludeMatcher,
		includeMatcher: includeMatcher,
		logger:         logger,
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

	// Filter out logs
	rLogs.RemoveIf(func(rl plog.ResourceLogs) bool {
		resource := rl.Resource()
		rl.ScopeLogs().RemoveIf(func(sl plog.ScopeLogs) bool {
			scope := sl.Scope()
			lrs := sl.LogRecords()

			if flp.includeMatcher != nil {
				// If includeMatcher exists, remove all records that do not match the filter.
				lrs.RemoveIf(func(lr plog.LogRecord) bool {
					return !flp.includeMatcher.MatchLogRecord(lr, resource, scope)
				})
			}

			if flp.excludeMatcher != nil {
				// If excludeMatcher exists, remove all records that match the filter.
				lrs.RemoveIf(func(lr plog.LogRecord) bool {
					return flp.excludeMatcher.MatchLogRecord(lr, resource, scope)
				})
			}
			return sl.LogRecords().Len() == 0
		})
		return rl.ScopeLogs().Len() == 0
	})

	if rLogs.Len() == 0 {
		return logs, processorhelper.ErrSkipProcessingData
	}

	return logs, nil
}
