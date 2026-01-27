// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/sampling"

import (
	"context"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/pkg/samplingpolicy"
)

type subpolicy struct {
	// the subpolicy evaluator
	evaluator samplingpolicy.Evaluator

	// spans per second allocated to each subpolicy
	allocatedSPS int64

	// spans per second that each subpolicy sampled in this period
	sampledSPS int64

	name string
}

// Composite evaluator and its internal data
type Composite struct {
	// the subpolicy evaluators
	subpolicies []*subpolicy

	// maximum total spans per second that must be sampled
	maxTotalSPS int64

	// current unix timestamp second
	currentSecond int64

	// The time provider (can be different from clock for testing purposes)
	timeProvider TimeProvider

	logger          *zap.Logger
	recordSubPolicy bool
}

var _ samplingpolicy.Evaluator = (*Composite)(nil)

// SubPolicyEvalParams defines the evaluator and max rate for a sub-policy
type SubPolicyEvalParams struct {
	Evaluator         samplingpolicy.Evaluator
	MaxSpansPerSecond int64
	Name              string
}

// NewComposite creates a policy evaluator that samples all subpolicies.
func NewComposite(
	logger *zap.Logger,
	maxTotalSpansPerSecond int64,
	subPolicyParams []SubPolicyEvalParams,
	timeProvider TimeProvider,
	recordSubPolicy bool,
) samplingpolicy.Evaluator {
	var subpolicies []*subpolicy

	for i := range subPolicyParams {
		sub := &subpolicy{}
		sub.evaluator = subPolicyParams[i].Evaluator
		sub.allocatedSPS = subPolicyParams[i].MaxSpansPerSecond
		sub.name = subPolicyParams[i].Name
		// We are just starting, so there is no previous input, set it to 0
		sub.sampledSPS = 0

		subpolicies = append(subpolicies, sub)
	}

	return &Composite{
		maxTotalSPS:     maxTotalSpansPerSecond,
		subpolicies:     subpolicies,
		timeProvider:    timeProvider,
		logger:          logger,
		recordSubPolicy: recordSubPolicy,
	}
}

// Evaluate looks at the trace data and returns a corresponding SamplingDecision.
func (c *Composite) Evaluate(ctx context.Context, traceID pcommon.TraceID, trace *samplingpolicy.TraceData) (samplingpolicy.Decision, error) {
	// Rate limiting works by counting spans that are sampled during each 1 second
	// time period. Until the total number of spans during a particular second
	// exceeds the allocated number of spans-per-second the traces are sampled,
	// once the limit is exceeded the traces are no longer sampled. The counter
	// restarts at the beginning of each second.
	// Current counters and rate limits are kept separately for each subpolicy.

	currSecond := c.timeProvider.getCurSecond()
	if c.currentSecond != currSecond {
		// This is a new second
		c.currentSecond = currSecond
		// Reset counters
		for i := range c.subpolicies {
			c.subpolicies[i].sampledSPS = 0
		}
	}

	for _, sub := range c.subpolicies {
		decision, err := sub.evaluator.Evaluate(ctx, traceID, trace)
		if err != nil {
			return samplingpolicy.Unspecified, err
		}

		if decision == samplingpolicy.Sampled || decision == samplingpolicy.InvertSampled {
			// The subpolicy made a decision to Sample. Now we need to make our decision.

			// Calculate resulting SPS counter if we decide to sample this trace
			spansInSecondIfSampled := sub.sampledSPS + trace.SpanCount

			// Check if the rate will be within the allocated bandwidth.
			if spansInSecondIfSampled <= sub.allocatedSPS && spansInSecondIfSampled <= c.maxTotalSPS {
				sub.sampledSPS = spansInSecondIfSampled

				// Let the sampling happen
				if c.recordSubPolicy {
					SetAttrOnScopeSpans(trace.ReceivedBatches, "tailsampling.composite_policy", sub.name)
				}
				return samplingpolicy.Sampled, nil
			}

			// We exceeded the rate limit. Don't sample this trace.
			// Note that we will continue evaluating new incoming traces against
			// allocated SPS, we do not update sub.sampledSPS here in order to give
			// chance to another smaller trace to be accepted later.
			return samplingpolicy.NotSampled, nil
		}
	}

	return samplingpolicy.NotSampled, nil
}
