// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package spanpruningprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/spanpruningprocessor"

import (
	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
)

// standardSpanFuncs returns the standard OTTL functions for span context.
// This includes all standard converters plus the IsRootSpan function.
func standardSpanFuncs() map[string]ottl.Factory[*ottlspan.TransformContext] {
	m := ottlfuncs.StandardConverters[*ottlspan.TransformContext]()
	isRootSpanFactory := ottlfuncs.NewIsRootSpanFactoryNew()
	m[isRootSpanFactory.Name()] = isRootSpanFactory
	return m
}

// newBoolExprForSpan creates a ConditionSequence that evaluates OTTL conditions against spans.
// This is equivalent to filterottl.NewBoolExprForSpan but uses only public APIs.
func newBoolExprForSpan(
	conditions []string,
	functions map[string]ottl.Factory[*ottlspan.TransformContext],
	errorMode ottl.ErrorMode,
	set component.TelemetrySettings,
) (*ottl.ConditionSequence[*ottlspan.TransformContext], error) {
	parser, err := ottlspan.NewParser(functions, set)
	if err != nil {
		return nil, err
	}
	statements, err := parser.ParseConditions(conditions)
	if err != nil {
		return nil, err
	}
	c := ottlspan.NewConditionSequence(statements, set, ottlspan.WithConditionSequenceErrorMode(errorMode))
	return &c, nil
}
