// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package adaptivetailsamplingprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/adaptivetailsamplingprocessor"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/adaptivetailsamplingprocessor/internal/sampler"
)

// rule is a compiled rule: OTTL conditions, a match mode, and the sampler to
// invoke when the rule matches.
type rule struct {
	name        string
	conditions  []*ottl.Condition[*ottlspan.TransformContext]
	matchMode   MatchMode
	sampler     sampler.Sampler
	fingerprint []sampler.Selector
	// needsRootMatcher is precomputed so evaluate only constructs the
	// ctx-capturing root matcher closure when a root.-scoped selector exists.
	needsRootMatcher bool

	logger      *zap.Logger
	evalErrs    metric.Int64Counter
	ruleAttrSet metric.MeasurementOption
}

// matches returns true when the rule's conditions are satisfied by the
// accumulated trace under the configured match mode. A rule with zero
// conditions is a catch-all and always matches.
func (r *rule) matches(ctx context.Context, spans []ptrace.ResourceSpans) bool {
	if len(r.conditions) == 0 {
		return true
	}
	switch r.matchMode {
	case MatchSameSpan:
		return r.matchesSameSpan(ctx, spans)
	default:
		return r.matchesAnySpan(ctx, spans)
	}
}

// matchesSameSpan returns true if some single span in the trace satisfies all
// of the rule's conditions.
func (r *rule) matchesSameSpan(ctx context.Context, spans []ptrace.ResourceSpans) bool {
	for _, rs := range spans {
		for _, ss := range rs.ScopeSpans().All() {
			for _, span := range ss.Spans().All() {
				tCtx := ottlspan.NewTransformContextPtr(rs, ss, span)
				allMatch := true
				for _, cond := range r.conditions {
					ok, err := cond.Eval(ctx, tCtx)
					if err != nil {
						r.recordEvalErr(ctx, err)
						allMatch = false
						break
					}
					if !ok {
						allMatch = false
						break
					}
				}
				tCtx.Close()
				if allMatch {
					return true
				}
			}
		}
	}
	return false
}

// matchesAnySpan returns true if every condition is satisfied by at least one
// span in the trace, not necessarily the same span across conditions.
func (r *rule) matchesAnySpan(ctx context.Context, spans []ptrace.ResourceSpans) bool {
	for _, cond := range r.conditions {
		if !r.anySpanSatisfies(ctx, spans, cond) {
			return false
		}
	}
	return true
}

func (r *rule) anySpanSatisfies(ctx context.Context, spans []ptrace.ResourceSpans, cond *ottl.Condition[*ottlspan.TransformContext]) bool {
	for _, rs := range spans {
		for _, ss := range rs.ScopeSpans().All() {
			for _, span := range ss.Spans().All() {
				tCtx := ottlspan.NewTransformContextPtr(rs, ss, span)
				ok, err := cond.Eval(ctx, tCtx)
				tCtx.Close()
				if err != nil {
					r.recordEvalErr(ctx, err)
					continue
				}
				if ok {
					return true
				}
			}
		}
	}
	return false
}

func (r *rule) recordEvalErr(ctx context.Context, err error) {
	if r.evalErrs != nil {
		r.evalErrs.Add(ctx, 1, r.ruleAttrSet)
	}
	if r.logger != nil {
		r.logger.Debug("OTTL condition evaluation failed",
			zap.String("rule", r.name),
			zap.Error(err))
	}
}

// compileRule turns a config rule into a runtime rule. The sampler must be
// supplied by the caller because constructing it depends on processor-wide
// resources.
func compileRule(cfg *RuleConfig, s sampler.Sampler, fingerprint []sampler.Selector, settings component.TelemetrySettings, evalErrs metric.Int64Counter) (*rule, error) {
	matchMode := cfg.Match
	if matchMode == "" {
		matchMode = MatchAnySpan
	}
	needsRoot := false
	for _, sel := range fingerprint {
		if sel.Scope == sampler.ScopeRoot {
			needsRoot = true
			break
		}
	}
	r := &rule{
		name:             cfg.Name,
		sampler:          s,
		fingerprint:      fingerprint,
		needsRootMatcher: needsRoot,
		matchMode:        matchMode,
		logger:           settings.Logger,
		evalErrs:         evalErrs,
		ruleAttrSet:      metric.WithAttributes(attribute.String("rule", cfg.Name)),
	}
	if len(cfg.Conditions) == 0 {
		return r, nil
	}
	parser, err := ottlspan.NewParser(filterottl.StandardSpanFuncs(), settings, ottlspan.EnablePathContextNames())
	if err != nil {
		return nil, fmt.Errorf("rule %q: build OTTL parser: %w", cfg.Name, err)
	}
	r.conditions = make([]*ottl.Condition[*ottlspan.TransformContext], 0, len(cfg.Conditions))
	for i, expr := range cfg.Conditions {
		cond, err := parser.ParseCondition(expr)
		if err != nil {
			return nil, fmt.Errorf("rule %q condition[%d]: %w", cfg.Name, i, err)
		}
		r.conditions = append(r.conditions, cond)
	}
	return r, nil
}
