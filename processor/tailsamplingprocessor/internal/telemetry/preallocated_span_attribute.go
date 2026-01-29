// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package telemetry // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/telemetry"

import (
	"go.opentelemetry.io/otel/attribute"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/pkg/samplingpolicy"
)

// DecisionAttributes holds pre-allocated attribute.KeyValue for all decision types.
// It provides O(1) access to pre-computed attributes using array indexing instead of map lookups,
// eliminating runtime allocations during trace processing. The struct is immutable after construction
// and safe for concurrent access.
type DecisionAttributes struct {
	key         attribute.Key         // The attribute key used for all decisions
	attrs       [8]attribute.KeyValue // Array indexed by Decision value (0-7)
	unknownAttr attribute.KeyValue    // Returned for invalid decisions
}

// NewDecisionAttributes creates a DecisionAttributes instance with pre-computed KeyValue pairs
// for all 8 decision types defined in samplingpolicy.Decision.
// Parameters:
//   - attrKey: The attribute key to use for all decision attributes (e.g., "decision.final", "policy.decision")
//
// Example:
//
//	spanDecisionAttrs := NewDecisionAttributes(attribute.Key("decision.final"))
//	attr := spanDecisionAttrs.Get(samplingpolicy.Sampled) // Returns pre-allocated KeyValue
func NewDecisionAttributes(attrKey attribute.Key) *DecisionAttributes {
	da := &DecisionAttributes{
		key: attrKey,
	}

	// Pre-compute all 8 decision attributes
	da.attrs[samplingpolicy.Unspecified] = attrKey.String(samplingpolicy.Unspecified.String())
	da.attrs[samplingpolicy.Pending] = attrKey.String(samplingpolicy.Pending.String())
	da.attrs[samplingpolicy.Sampled] = attrKey.String(samplingpolicy.Sampled.String())
	da.attrs[samplingpolicy.NotSampled] = attrKey.String(samplingpolicy.NotSampled.String())
	da.attrs[samplingpolicy.Dropped] = attrKey.String(samplingpolicy.Dropped.String())
	da.attrs[samplingpolicy.Error] = attrKey.String(samplingpolicy.Error.String())
	da.attrs[samplingpolicy.InvertSampled] = attrKey.String(samplingpolicy.InvertSampled.String())
	da.attrs[samplingpolicy.InvertNotSampled] = attrKey.String(samplingpolicy.InvertNotSampled.String())

	// Pre-compute the unknown attribute for out-of-bounds decisions
	da.unknownAttr = attrKey.String("missing.preallocation")

	return da
}

// Get returns the pre-allocated KeyValue for the given decision.
func (d *DecisionAttributes) Get(decision samplingpolicy.Decision) attribute.KeyValue {
	// Bounds check: valid decisions are 0-7 (inclusive)
	if decision < 0 || decision >= samplingpolicy.Decision(len(d.attrs)) {
		return d.unknownAttr
	}
	return d.attrs[decision]
}
