// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filterottl // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterottl"

import (
	"slices"

	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottldatapoint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlexemplar"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlresource"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlscope"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspanevent"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/xprofile/ottlprofile"
)

// NewBoolExprForSpan creates a BoolExpr[*ottlspan.TransformContext] that will return true if any of the given OTTL conditions evaluate to true.
// The passed in functions should use the ottlspan.TransformContext.
// If a function named `match` is not present in the function map it will be added automatically so that parsing works as expected
func NewBoolExprForSpan(conditions []string, functions map[string]ottl.Factory[*ottlspan.TransformContext], errorMode ottl.ErrorMode, set component.TelemetrySettings) (*ottl.ConditionSequence[*ottlspan.TransformContext], error) {
	return NewBoolExprForSpanWithOptions(conditions, functions, errorMode, set, nil)
}

// NewBoolExprForSpanWithOptions is like NewBoolExprForSpan, but with additional options.
func NewBoolExprForSpanWithOptions(conditions []string, functions map[string]ottl.Factory[*ottlspan.TransformContext], errorMode ottl.ErrorMode, set component.TelemetrySettings, parserOptions []ottl.Option[*ottlspan.TransformContext], sequenceOptions ...ottlspan.ConditionSequenceOption) (*ottl.ConditionSequence[*ottlspan.TransformContext], error) {
	parser, err := ottlspan.NewParser(functions, set, parserOptions...)
	if err != nil {
		return nil, err
	}
	statements, err := parser.ParseConditions(conditions)
	if err != nil {
		return nil, err
	}
	c := ottlspan.NewConditionSequence(statements, set, slices.Concat(sequenceOptions, []ottlspan.ConditionSequenceOption{ottlspan.WithConditionSequenceErrorMode(errorMode)})...)
	return &c, nil
}

// NewBoolExprForSpanEvent creates a BoolExpr[*ottlspanevent.TransformContext] that will return true if any of the given OTTL conditions evaluate to true.
// The passed in functions should use the ottlspanevent.TransformContext.
// If a function named `match` is not present in the function map it will be added automatically so that parsing works as expected
func NewBoolExprForSpanEvent(conditions []string, functions map[string]ottl.Factory[*ottlspanevent.TransformContext], errorMode ottl.ErrorMode, set component.TelemetrySettings) (*ottl.ConditionSequence[*ottlspanevent.TransformContext], error) {
	return NewBoolExprForSpanEventWithOptions(conditions, functions, errorMode, set, nil)
}

// NewBoolExprForSpanEventWithOptions is like NewBoolExprForSpanEvent, but with additional options.
func NewBoolExprForSpanEventWithOptions(conditions []string, functions map[string]ottl.Factory[*ottlspanevent.TransformContext], errorMode ottl.ErrorMode, set component.TelemetrySettings, parserOptions []ottl.Option[*ottlspanevent.TransformContext], sequenceOptions ...ottlspanevent.ConditionSequenceOption) (*ottl.ConditionSequence[*ottlspanevent.TransformContext], error) {
	parser, err := ottlspanevent.NewParser(functions, set, parserOptions...)
	if err != nil {
		return nil, err
	}
	statements, err := parser.ParseConditions(conditions)
	if err != nil {
		return nil, err
	}
	c := ottlspanevent.NewConditionSequence(statements, set, slices.Concat(sequenceOptions, []ottlspanevent.ConditionSequenceOption{ottlspanevent.WithConditionSequenceErrorMode(errorMode)})...)
	return &c, nil
}

// NewBoolExprForMetric creates a BoolExpr[*ottlmetric.TransformContext] that will return true if any of the given OTTL conditions evaluate to true.
// The passed in functions should use the ottlmetric.TransformContext.
// If a function named `match` is not present in the function map it will be added automatically so that parsing works as expected
func NewBoolExprForMetric(conditions []string, functions map[string]ottl.Factory[*ottlmetric.TransformContext], errorMode ottl.ErrorMode, set component.TelemetrySettings) (*ottl.ConditionSequence[*ottlmetric.TransformContext], error) {
	return NewBoolExprForMetricWithOptions(conditions, functions, errorMode, set, nil)
}

// NewBoolExprForMetricWithOptions is like NewBoolExprForMetric, but with additional options.
func NewBoolExprForMetricWithOptions(conditions []string, functions map[string]ottl.Factory[*ottlmetric.TransformContext], errorMode ottl.ErrorMode, set component.TelemetrySettings, parserOptions []ottl.Option[*ottlmetric.TransformContext], sequenceOptions ...ottlmetric.ConditionSequenceOption) (*ottl.ConditionSequence[*ottlmetric.TransformContext], error) {
	parser, err := ottlmetric.NewParser(functions, set, parserOptions...)
	if err != nil {
		return nil, err
	}
	statements, err := parser.ParseConditions(conditions)
	if err != nil {
		return nil, err
	}
	c := ottlmetric.NewConditionSequence(statements, set, slices.Concat(sequenceOptions, []ottlmetric.ConditionSequenceOption{ottlmetric.WithConditionSequenceErrorMode(errorMode)})...)
	return &c, nil
}

// NewBoolExprForDataPoint creates a BoolExpr[*ottldatapoint.TransformContext] that will return true if any of the given OTTL conditions evaluate to true.
// The passed in functions should use the ottldatapoint.TransformContext.
// If a function named `match` is not present in the function map it will be added automatically so that parsing works as expected
func NewBoolExprForDataPoint(conditions []string, functions map[string]ottl.Factory[*ottldatapoint.TransformContext], errorMode ottl.ErrorMode, set component.TelemetrySettings) (*ottl.ConditionSequence[*ottldatapoint.TransformContext], error) {
	return NewBoolExprForDataPointWithOptions(conditions, functions, errorMode, set, nil)
}

// NewBoolExprForDataPointWithOptions is like NewBoolExprForDataPoint, but with additional options.
func NewBoolExprForDataPointWithOptions(conditions []string, functions map[string]ottl.Factory[*ottldatapoint.TransformContext], errorMode ottl.ErrorMode, set component.TelemetrySettings, parserOptions []ottl.Option[*ottldatapoint.TransformContext], sequenceOptions ...ottldatapoint.ConditionSequenceOption) (*ottl.ConditionSequence[*ottldatapoint.TransformContext], error) {
	parser, err := ottldatapoint.NewParser(functions, set, parserOptions...)
	if err != nil {
		return nil, err
	}
	statements, err := parser.ParseConditions(conditions)
	if err != nil {
		return nil, err
	}
	c := ottldatapoint.NewConditionSequence(statements, set, slices.Concat(sequenceOptions, []ottldatapoint.ConditionSequenceOption{ottldatapoint.WithConditionSequenceErrorMode(errorMode)})...)
	return &c, nil
}

// NewBoolExprForExemplar creates a BoolExpr[*ottlexemplar.TransformContext] that will return true if any of the given OTTL conditions evaluate to true.
// The passed in functions should use the ottlexemplar.TransformContext.
// If a function named `match` is not present in the function map it will be added automatically so that parsing works as expected
func NewBoolExprForExemplar(conditions []string, functions map[string]ottl.Factory[*ottlexemplar.TransformContext], errorMode ottl.ErrorMode, set component.TelemetrySettings) (*ottl.ConditionSequence[*ottlexemplar.TransformContext], error) {
	return NewBoolExprForExemplarWithOptions(conditions, functions, errorMode, set, nil)
}

// NewBoolExprForExemplarWithOptions is like NewBoolExprForExemplar, but with additional options.
func NewBoolExprForExemplarWithOptions(conditions []string, functions map[string]ottl.Factory[*ottlexemplar.TransformContext], errorMode ottl.ErrorMode, set component.TelemetrySettings, parserOptions []ottl.Option[*ottlexemplar.TransformContext], sequenceOptions ...ottlexemplar.ConditionSequenceOption) (*ottl.ConditionSequence[*ottlexemplar.TransformContext], error) {
	parser, err := ottlexemplar.NewParser(functions, set, parserOptions...)
	if err != nil {
		return nil, err
	}
	statements, err := parser.ParseConditions(conditions)
	if err != nil {
		return nil, err
	}
	c := ottlexemplar.NewConditionSequence(statements, set, slices.Concat(sequenceOptions, []ottlexemplar.ConditionSequenceOption{ottlexemplar.WithConditionSequenceErrorMode(errorMode)})...)
	return &c, nil
}

// NewBoolExprForLog creates a BoolExpr[*ottllog.TransformContext] that will return true if any of the given OTTL conditions evaluate to true.
// The passed in functions should use the ottllog.TransformContext.
// If a function named `match` is not present in the function map it will be added automatically so that parsing works as expected
func NewBoolExprForLog(conditions []string, functions map[string]ottl.Factory[*ottllog.TransformContext], errorMode ottl.ErrorMode, set component.TelemetrySettings) (*ottl.ConditionSequence[*ottllog.TransformContext], error) {
	return NewBoolExprForLogWithOptions(conditions, functions, errorMode, set, nil)
}

// NewBoolExprForLogWithOptions is like NewBoolExprForLog, but with additional options.
func NewBoolExprForLogWithOptions(conditions []string, functions map[string]ottl.Factory[*ottllog.TransformContext], errorMode ottl.ErrorMode, set component.TelemetrySettings, parserOptions []ottl.Option[*ottllog.TransformContext], sequenceOptions ...ottllog.ConditionSequenceOption) (*ottl.ConditionSequence[*ottllog.TransformContext], error) {
	parser, err := ottllog.NewParser(functions, set, parserOptions...)
	if err != nil {
		return nil, err
	}
	statements, err := parser.ParseConditions(conditions)
	if err != nil {
		return nil, err
	}
	c := ottllog.NewConditionSequence(statements, set, slices.Concat(sequenceOptions, []ottllog.ConditionSequenceOption{ottllog.WithConditionSequenceErrorMode(errorMode)})...)
	return &c, nil
}

// NewBoolExprForProfile creates a BoolExpr[*ottlprofile.TransformContext] that will return true if any of the given OTTL conditions evaluate to true.
// The passed in functions should use the ottlprofile.TransformContext.
// If a function named `match` is not present in the function map it will be added automatically so that parsing works as expected
func NewBoolExprForProfile(conditions []string, functions map[string]ottl.Factory[*ottlprofile.TransformContext], errorMode ottl.ErrorMode, set component.TelemetrySettings) (*ottl.ConditionSequence[*ottlprofile.TransformContext], error) {
	return NewBoolExprForProfileWithOptions(conditions, functions, errorMode, set, nil)
}

// NewBoolExprForProfileWithOptions is like NewBoolExprForProfile, but with additional options.
func NewBoolExprForProfileWithOptions(conditions []string, functions map[string]ottl.Factory[*ottlprofile.TransformContext], errorMode ottl.ErrorMode, set component.TelemetrySettings, parserOptions []ottl.Option[*ottlprofile.TransformContext], sequenceOptions ...ottlprofile.ConditionSequenceOption) (*ottl.ConditionSequence[*ottlprofile.TransformContext], error) {
	parser, err := ottlprofile.NewParser(functions, set, parserOptions...)
	if err != nil {
		return nil, err
	}
	statements, err := parser.ParseConditions(conditions)
	if err != nil {
		return nil, err
	}
	c := ottlprofile.NewConditionSequence(statements, set, slices.Concat(sequenceOptions, []ottlprofile.ConditionSequenceOption{ottlprofile.WithConditionSequenceErrorMode(errorMode)})...)
	return &c, nil
}

// NewBoolExprForResource creates a BoolExpr[*ottlresource.TransformContext] that will return true if any of the given OTTL conditions evaluate to true.
// The passed in functions should use the ottlresource.TransformContext.
// If a function named `match` is not present in the function map it will be added automatically so that parsing works as expected
func NewBoolExprForResource(conditions []string, functions map[string]ottl.Factory[*ottlresource.TransformContext], errorMode ottl.ErrorMode, set component.TelemetrySettings) (*ottl.ConditionSequence[*ottlresource.TransformContext], error) {
	return NewBoolExprForResourceWithOptions(conditions, functions, errorMode, set, nil)
}

// NewBoolExprForResourceWithOptions is like NewBoolExprForResource, but with additional options.
func NewBoolExprForResourceWithOptions(conditions []string, functions map[string]ottl.Factory[*ottlresource.TransformContext], errorMode ottl.ErrorMode, set component.TelemetrySettings, parserOptions []ottl.Option[*ottlresource.TransformContext], sequenceOptions ...ottlresource.ConditionSequenceOption) (*ottl.ConditionSequence[*ottlresource.TransformContext], error) {
	parser, err := ottlresource.NewParser(functions, set, parserOptions...)
	if err != nil {
		return nil, err
	}
	statements, err := parser.ParseConditions(conditions)
	if err != nil {
		return nil, err
	}
	c := ottlresource.NewConditionSequence(statements, set, slices.Concat(sequenceOptions, []ottlresource.ConditionSequenceOption{ottlresource.WithConditionSequenceErrorMode(errorMode)})...)
	return &c, nil
}

// NewBoolExprForScope creates a BoolExpr[*ottlscope.TransformContext] that will return true if any of the given OTTL conditions evaluate to true.
// The passed in functions should use the ottlresource.TransformContext.
// If a function named `match` is not present in the function map it will be added automatically so that parsing works as expected
func NewBoolExprForScope(conditions []string, functions map[string]ottl.Factory[*ottlscope.TransformContext], errorMode ottl.ErrorMode, set component.TelemetrySettings) (*ottl.ConditionSequence[*ottlscope.TransformContext], error) {
	return NewBoolExprForScopeWithOptions(conditions, functions, errorMode, set, nil)
}

// NewBoolExprForScopeWithOptions is like NewBoolExprForScope, but with additional options.
func NewBoolExprForScopeWithOptions(conditions []string, functions map[string]ottl.Factory[*ottlscope.TransformContext], errorMode ottl.ErrorMode, set component.TelemetrySettings, parserOptions []ottl.Option[*ottlscope.TransformContext], sequenceOptions ...ottlscope.ConditionSequenceOption) (*ottl.ConditionSequence[*ottlscope.TransformContext], error) {
	parser, err := ottlscope.NewParser(functions, set, parserOptions...)
	if err != nil {
		return nil, err
	}
	statements, err := parser.ParseConditions(conditions)
	if err != nil {
		return nil, err
	}
	c := ottlscope.NewConditionSequence(statements, set, slices.Concat(sequenceOptions, []ottlscope.ConditionSequenceOption{ottlscope.WithConditionSequenceErrorMode(errorMode)})...)
	return &c, nil
}
