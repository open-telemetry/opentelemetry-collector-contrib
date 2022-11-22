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

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor/processorhelper"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterspan"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspanevent"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor/internal/common"
)

type filterSpanProcessor struct {
	cfg                 *Config
	include             filterspan.Matcher
	exclude             filterspan.Matcher
	logger              *zap.Logger
	spanConditions      []*ottl.Statement[ottlspan.TransformContext]
	spanEventConditions []*ottl.Statement[ottlspanevent.TransformContext]
}

func newFilterSpansProcessor(logger *zap.Logger, cfg *Config) (*filterSpanProcessor, error) {
	if cfg.Traces.SpanConditions != nil || cfg.Traces.SpanEventConditions != nil {
		fsp := &filterSpanProcessor{
			cfg:    cfg,
			logger: logger,
		}

		if cfg.Traces.SpanConditions != nil {
			spanp := ottlspan.NewParser(common.Functions[ottlspan.TransformContext](), component.TelemetrySettings{Logger: zap.NewNop()})
			statements, err := spanp.ParseStatements(common.PrepareConditionForParsing(cfg.Traces.SpanConditions))
			if err != nil {
				return nil, err
			}
			fsp.spanConditions = statements
		}

		if cfg.Traces.SpanEventConditions != nil {
			spaneventp := ottlspanevent.NewParser(common.Functions[ottlspanevent.TransformContext](), component.TelemetrySettings{Logger: zap.NewNop()})
			statements, err := spaneventp.ParseStatements(common.PrepareConditionForParsing(cfg.Traces.SpanEventConditions))
			if err != nil {
				return nil, err
			}
			fsp.spanEventConditions = statements
		}
		return fsp, nil
	}

	if cfg.Spans.Include == nil && cfg.Spans.Exclude == nil {
		return nil, nil
	}

	inc, exc, err := createSpanMatcher(cfg)
	if err != nil {
		return nil, err
	}

	includeMatchType, excludeMatchType := "[None]", "[None]"
	if cfg.Spans.Include != nil {
		includeMatchType = string(cfg.Spans.Include.MatchType)
	}

	if cfg.Spans.Exclude != nil {
		excludeMatchType = string(cfg.Spans.Exclude.MatchType)
	}

	logger.Info(
		"Span filter configured",
		zap.String("ID", cfg.ID().String()),
		zap.String("[Include] match_type", includeMatchType),
		zap.String("[Exclude] match_type", excludeMatchType),
	)

	return &filterSpanProcessor{
		cfg:     cfg,
		include: inc,
		exclude: exc,
		logger:  logger,
	}, nil
}

func createSpanMatcher(cfg *Config) (filterspan.Matcher, filterspan.Matcher, error) {
	var includeMatcher filterspan.Matcher
	var excludeMatcher filterspan.Matcher
	var err error
	if cfg.Spans.Include != nil {
		includeMatcher, err = filterspan.NewMatcher(cfg.Spans.Include)
		if err != nil {
			return nil, nil, err
		}
	}
	if cfg.Spans.Exclude != nil {
		excludeMatcher, err = filterspan.NewMatcher(cfg.Spans.Exclude)
		if err != nil {
			return nil, nil, err
		}
	}
	return includeMatcher, excludeMatcher, nil
}

// processTraces filters the given spans of a traces based off the filterSpanProcessor's filters.
func (fsp *filterSpanProcessor) processTraces(ctx context.Context, pdt ptrace.Traces) (ptrace.Traces, error) {
	filteringSpans := fsp.spanConditions != nil
	filteringSpanEvents := fsp.spanEventConditions != nil

	if filteringSpans || filteringSpanEvents {
		var errors error
		pdt.ResourceSpans().RemoveIf(func(rspans ptrace.ResourceSpans) bool {
			rspans.ScopeSpans().RemoveIf(func(sspans ptrace.ScopeSpans) bool {
				sspans.Spans().RemoveIf(func(span ptrace.Span) bool {
					if filteringSpans {
						tCtx := ottlspan.NewTransformContext(span, sspans.Scope(), rspans.Resource())
						metCondition, err := common.CheckConditions(ctx, tCtx, fsp.spanConditions)
						if err != nil {
							errors = multierr.Append(errors, err)
							return false
						}
						if metCondition {
							return true
						}
					}
					if filteringSpanEvents {
						span.Events().RemoveIf(func(spanEvent ptrace.SpanEvent) bool {
							tCtx := ottlspanevent.NewTransformContext(spanEvent, span, sspans.Scope(), rspans.Resource())
							metCondition, err := common.CheckConditions(ctx, tCtx, fsp.spanEventConditions)
							if err != nil {
								errors = multierr.Append(errors, err)
								return false
							}
							return metCondition
						})
					}
					return false
				})
				return sspans.Spans().Len() == 0
			})
			return rspans.ScopeSpans().Len() == 0
		})

		if errors != nil {
			return pdt, errors
		}
		if pdt.ResourceSpans().Len() == 0 {
			return pdt, processorhelper.ErrSkipProcessingData
		}
		return pdt, nil
	}

	pdt.ResourceSpans().RemoveIf(func(rs ptrace.ResourceSpans) bool {
		resource := rs.Resource()
		rs.ScopeSpans().RemoveIf(func(ss ptrace.ScopeSpans) bool {
			scope := ss.Scope()
			ss.Spans().RemoveIf(func(span ptrace.Span) bool {
				if fsp.include != nil {
					if !fsp.include.MatchSpan(span, resource, scope) {
						return true
					}
				}

				if fsp.exclude != nil {
					if fsp.exclude.MatchSpan(span, resource, scope) {
						return true
					}
				}

				return false
			})
			return ss.Spans().Len() == 0
		})
		return rs.ScopeSpans().Len() == 0
	})
	if pdt.ResourceSpans().Len() == 0 {
		return pdt, processorhelper.ErrSkipProcessingData
	}
	return pdt, nil
}
