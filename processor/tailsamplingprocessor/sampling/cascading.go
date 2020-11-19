// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sampling

import (
	"context"
	"errors"
	"time"

	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/config"
)

type cascadingPolicy struct {
	logger *zap.Logger

	currentSecond        int64
	maxSpansPerSecond    int64
	spansInCurrentSecond int64

	rules []*cascadingRuleEvaluation
}

type cascadingRuleEvaluation struct {
	// In fact, this can be only NumericTagFilter, StringTagFilter, Properties or AlwaysSample
	evaluator PolicyEvaluator

	context context.Context

	currentSecond int64
	// Set maxSpansPerSecond to zero to make it opportunistic rule (it will fill
	// only if there's space after evaluating other ones)
	maxSpansPerSecond    int64
	spansInCurrentSecond int64
}

var (
	tagRuleKey, _       = tag.NewKey("rule")
	tagSampledKey, _    = tag.NewKey("sampled")
	tagConsideredKey, _ = tag.NewKey("considered")

	statCascadingTracesSampled = stats.Int64("count_cascading_traces_sampled", "Count of traces that were sampled or not", stats.UnitDimensionless)

	sampledTagKeys              = []tag.Key{tagRuleKey, tagConsideredKey, tagSampledKey}
	CountTracesSampledRulesView = &view.View{
		Name:        statCascadingTracesSampled.Name(),
		Measure:     statCascadingTracesSampled,
		Description: statCascadingTracesSampled.Description(),
		TagKeys:     sampledTagKeys,
		Aggregation: view.Sum(),
	}
)

func (cp *cascadingPolicy) shouldConsider(currSecond int64, trace *TraceData) bool {
	if trace.SpanCount > cp.maxSpansPerSecond {
		// This trace will never fit
		return false
	} else if cp.currentSecond == currSecond && trace.SpanCount > cp.maxSpansPerSecond-cp.spansInCurrentSecond {
		// This trace will not fit in this second, no way
		return false
	} else {
		return true
	}
}

func (cp *cascadingPolicy) updateRate(currSecond int64, numSpans int64) Decision {
	if cp.currentSecond != currSecond {
		cp.currentSecond = currSecond
		cp.spansInCurrentSecond = 0
	}

	spansInSecondIfSampled := cp.spansInCurrentSecond + numSpans
	if spansInSecondIfSampled <= cp.maxSpansPerSecond {
		cp.spansInCurrentSecond = spansInSecondIfSampled
		return Sampled
	}

	return NotSampled
}

func (cre *cascadingRuleEvaluation) updateRate(currSecond int64, trace *TraceData) Decision {
	if cre.currentSecond != currSecond {
		cre.currentSecond = currSecond
		cre.spansInCurrentSecond = 0
	}

	spansInSecondIfSampled := cre.spansInCurrentSecond + trace.SpanCount
	if spansInSecondIfSampled <= cre.maxSpansPerSecond {
		cre.spansInCurrentSecond = spansInSecondIfSampled
		return Sampled
	}

	return NotSampled
}

var _ PolicyEvaluator = (*cascadingPolicy)(nil)

// NewNumericAttributeFilter creates a cascading policy evaluator
func NewCascadingFilter(logger *zap.Logger, cfg *config.PolicyCfg) (PolicyEvaluator, error) {
	cascadingRules := make([]*cascadingRuleEvaluation, 0)

	for _, ruleCfg := range cfg.Rules {
		attributesSet := 0
		if ruleCfg.StringAttributeCfg != nil {
			attributesSet++
		}
		if ruleCfg.NumericAttributeCfg != nil {
			attributesSet++
		}
		if ruleCfg.PropertiesCfg != nil {
			attributesSet++
		}

		if attributesSet > 1 {
			return nil, errors.New("cascading policy can have at most one filter set per rule")
		}

		evaluator := NewAlwaysSample(logger)

		if ruleCfg.StringAttributeCfg != nil {
			evaluator = NewStringAttributeFilter(logger, ruleCfg.StringAttributeCfg.Key, ruleCfg.StringAttributeCfg.Values)
		} else if ruleCfg.NumericAttributeCfg != nil {
			evaluator = NewNumericAttributeFilter(logger,
				ruleCfg.NumericAttributeCfg.Key,
				ruleCfg.NumericAttributeCfg.MinValue,
				ruleCfg.NumericAttributeCfg.MaxValue)
		} else if ruleCfg.PropertiesCfg != nil {
			var err error
			evaluator, err = NewSpanPropertiesFilter(logger, ruleCfg.PropertiesCfg.NamePattern, ruleCfg.PropertiesCfg.MinDurationMicros, ruleCfg.PropertiesCfg.MinNumberOfSpans)
			if err != nil {
				return nil, err
			}
		}

		ctx, _ := tag.New(context.Background(), tag.Upsert(tagRuleKey, ruleCfg.Name))

		cascadingRules = append(cascadingRules, &cascadingRuleEvaluation{
			evaluator:         evaluator,
			context:           ctx,
			maxSpansPerSecond: ruleCfg.SpansPerSecond,
		})
	}

	return &cascadingPolicy{
		logger:            logger,
		maxSpansPerSecond: cfg.SpansPerSecond,
		rules:             cascadingRules,
	}, nil
}

// OnLateArrivingSpans notifies the evaluator that the given list of spans arrived
// after the sampling decision was already taken for the trace.
// This gives the evaluator a chance to log any message/metrics and/or update any
// related internal state.
func (cp *cascadingPolicy) OnLateArrivingSpans(earlyDecision Decision, spans []*pdata.Span) error {
	if earlyDecision == Sampled {
		// Update the current rate, this event means that spans were sampled nevertheless due to previous decision
		cp.updateRate(time.Now().Unix(), int64(len(spans)))
	}
	cp.logger.Debug("Triggering action for late arriving spans in cascading filter")
	return nil
}

// Evaluate looks at the trace data and returns a corresponding SamplingDecision.
func (cp *cascadingPolicy) Evaluate(traceID pdata.TraceID, trace *TraceData) (Decision, error) {
	cp.logger.Debug("Evaluating spans in cascading filter")

	currSecond := time.Now().Unix()

	if !cp.shouldConsider(currSecond, trace) {
		return NotSampled, nil
	}

	for _, rule := range cp.rules {
		if rule.maxSpansPerSecond < 0 {
			if ruleDecision, err := rule.evaluator.Evaluate(traceID, trace); err == nil && ruleDecision == Sampled {
				recordSampled(rule.context, "second_chance", SecondChance)
				return SecondChance, nil
			}
			recordSampled(rule.context, "second_chance", NotSampled)
		} else {
			if ruleDecision, err := rule.evaluator.Evaluate(traceID, trace); err == nil && ruleDecision == Sampled {
				// If it's here, then the criteria for the rule fit

				if rule.updateRate(currSecond, trace) == Sampled {
					// If it's here, then the trace made it into the sampled pool

					// This step is a sanity check (which might sample out the trace when
					// sampling is misconfigured or when some late spans from previous
					// decision got in already)
					policyDecision := cp.updateRate(currSecond, trace.SpanCount)
					recordSampled(rule.context, "matched", policyDecision)
					return policyDecision, nil
				}

				// This will ensure it's not sampled (otherwise it would get into SecondChance)
				// This behavior might be worth giving a deeper thought, as it effective removes matching
				// traces from the SecondChance pool.
				recordSampled(rule.context, "matched", NotSampled)
				return NotSampled, nil

			}
			recordSampled(rule.context, "not-matched", NotSampled)
		}
	}

	return NotSampled, nil
}

func recordSampled(ctx context.Context, consideredKey string, decision Decision) {
	decisionStr := "false"
	if decision == Sampled {
		decisionStr = "true"
	} else if decision == SecondChance {
		decisionStr = "second_chance"
	}

	stats.RecordWithTags(
		ctx,
		[]tag.Mutator{tag.Insert(tagConsideredKey, consideredKey), tag.Insert(tagSampledKey, decisionStr)},
		statCascadingTracesSampled.M(int64(1)),
	)

}

// EvaluateSecondChance looks if more traces can be fit after initial decisions was made
func (cp *cascadingPolicy) EvaluateSecondChance(_ pdata.TraceID, trace *TraceData) (Decision, error) {
	// Lets keep it evaluated for the current batch second
	sampled := cp.updateRate(cp.currentSecond, trace.SpanCount)
	recordSampled(context.Background(), "second_chance", sampled)
	return sampled, nil
}
