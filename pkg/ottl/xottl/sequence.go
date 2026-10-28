// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package xottl // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/xottl"

import (
	"go.opentelemetry.io/otel/metric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/internal/failurecounter"
)

// WithStatementSequenceFailureCounter sets a counter that is incremented by 1 each time a call to Execute
// encounters at least one statement error, regardless of the ErrorMode.
// A call to Execute in which several statements fail is counted once.
// The counter measures OTTL executions only, not records: Execute is called once per item of the
// OTTL context being processed (e.g. per span, per resource), so the rate depends on that context.
func WithStatementSequenceFailureCounter[K any](counter metric.Int64Counter) ottl.StatementSequenceOption[K] {
	return func(s *ottl.StatementSequence[K]) {
		s.SetFailureCounter(failurecounter.Counter{Int64Counter: counter})
	}
}

// WithConditionSequenceFailureCounter sets a counter that is incremented by 1 each time a call to Eval
// encounters at least one condition error, regardless of the ErrorMode.
// A call to Eval in which several conditions fail is counted once.
// The counter measures OTTL evaluations only, not records: Eval is called once per item of the
// OTTL context being processed (e.g. per span, per resource), so the rate depends on that context.
func WithConditionSequenceFailureCounter[K any](counter metric.Int64Counter) ottl.ConditionSequenceOption[K] {
	return func(c *ottl.ConditionSequence[K]) {
		c.SetFailureCounter(failurecounter.Counter{Int64Counter: counter})
	}
}
