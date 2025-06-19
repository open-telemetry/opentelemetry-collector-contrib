// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampler

import (
	"fmt"

	"go.opentelemetry.io/otel/trace"
)

type RuleBasedOption func(*ruleBasedConfig)

func WithRule(predicate Predicate, sampler ComposableSampler) RuleBasedOption {
	return func(rb *ruleBasedConfig) {
		rb.rules = append(rb.rules, ruleAndPredicate{
			Predicate:         predicate,
			ComposableSampler: sampler,
		})
	}
}

func WithDefaultRule(sampler ComposableSampler) RuleBasedOption {
	return func(rb *ruleBasedConfig) {
		rb.defRule = sampler
	}
}

type Predicate struct {
	function    func(ComposableSamplingParameters) bool
	description string
}

func NewPredicate(function func(ComposableSamplingParameters) bool, description string) Predicate {
	return Predicate{
		function:    function,
		description: description,
	}
}

func (p Predicate) Decide(params ComposableSamplingParameters) bool {
	if p.function == nil {
		return false
	}
	return p.function(params)
}

func (p Predicate) Description() string {
	return p.description
}

func TruePredicate() Predicate {
	return NewPredicate(func(params ComposableSamplingParameters) bool {
		return true
	}, "true")
}

func NegatePredicate(original Predicate) Predicate {
	return NewPredicate(func(params ComposableSamplingParameters) bool {
		return !original.function(params)
	}, fmt.Sprintf("not(%s)", original.description))
}

func SpanNamePredicate(name string) Predicate {
	return NewPredicate(func(params ComposableSamplingParameters) bool {
		return name == params.Name
	}, fmt.Sprintf("Span.Name==%s", name))
}

func SpanKindPredicate(kind trace.SpanKind) Predicate {
	return NewPredicate(func(params ComposableSamplingParameters) bool {
		return kind == params.Kind
	}, fmt.Sprintf("Span.Kind==%s", kind))
}

func IsRootPredicate() Predicate {
	return NewPredicate(func(params ComposableSamplingParameters) bool {
		return !params.ParentSpanContext.IsValid()
	}, "root?")
}

func IsRemotePredicate() Predicate {
	return NewPredicate(func(params ComposableSamplingParameters) bool {
		return params.ParentSpanContext.IsValid() && params.ParentSpanContext.IsRemote()
	}, "remote?")
}

func IsLocalPredicate() Predicate {
	return NewPredicate(func(params ComposableSamplingParameters) bool {
		return params.ParentSpanContext.IsValid() && !params.ParentSpanContext.IsRemote()
	}, "local?")
}
