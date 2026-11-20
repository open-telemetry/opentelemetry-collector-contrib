// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filterottl // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterottl"

import (
	"slices"

	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottldatapoint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlresource"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlscope"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspanevent"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/xprofile/ottlprofile"
)

// newBoolExprWithPathContextNames wraps parser in a single-context ottl.ParserCollection so
// that conditions without an explicit path context are rewritten to use contextName as their
// context. The parser must be constructed with EnablePathContextNames().
func newBoolExprWithPathContextNames[K any](
	contextName string,
	parser ottl.Parser[K],
	conditions []string,
	set component.TelemetrySettings,
	newConditionSequence func([]*ottl.Condition[K]) ottl.ConditionSequence[K],
) (*ottl.ConditionSequence[K], error) {
	pc, err := ottl.NewParserCollection(
		set,
		ottl.EnableParserCollectionModifiedPathsLogging[*ottl.ConditionSequence[K]](true),
		ottl.WithParserCollectionContext(
			contextName,
			&parser,
			ottl.WithConditionConverter(func(_ *ottl.ParserCollection[*ottl.ConditionSequence[K]], _ ottl.ConditionsGetter, parsed []*ottl.Condition[K]) (*ottl.ConditionSequence[K], error) {
				cs := newConditionSequence(parsed)
				return &cs, nil
			}),
		),
	)
	if err != nil {
		return nil, err
	}
	return pc.ParseConditionsWithContext(contextName, ottl.NewConditionsGetter(conditions), true)
}

// NewBoolExprForSpanWithPathContextNames is like NewBoolExprForSpan, but conditions may use OTTL
// path context names (e.g. `span.attributes["foo"]`). Conditions without an explicit context are
// rewritten to use the span context (e.g. `attributes["foo"]` becomes `span.attributes["foo"]`).
func NewBoolExprForSpanWithPathContextNames(conditions []string, functions map[string]ottl.Factory[*ottlspan.TransformContext], errorMode ottl.ErrorMode, set component.TelemetrySettings, sequenceOptions ...ottlspan.ConditionSequenceOption) (*ottl.ConditionSequence[*ottlspan.TransformContext], error) {
	parser, err := ottlspan.NewParser(functions, set, ottlspan.EnablePathContextNames())
	if err != nil {
		return nil, err
	}
	return newBoolExprWithPathContextNames(ottlspan.ContextName, parser, conditions, set, func(parsed []*ottl.Condition[*ottlspan.TransformContext]) ottl.ConditionSequence[*ottlspan.TransformContext] {
		return ottlspan.NewConditionSequence(parsed, set, slices.Concat(sequenceOptions, []ottlspan.ConditionSequenceOption{ottlspan.WithConditionSequenceErrorMode(errorMode)})...)
	})
}

// NewBoolExprForSpanEventWithPathContextNames is like NewBoolExprForSpanEvent, but conditions may use
// OTTL path context names (e.g. `spanevent.attributes["foo"]`). Conditions without an explicit
// context are rewritten to use the spanevent context.
func NewBoolExprForSpanEventWithPathContextNames(conditions []string, functions map[string]ottl.Factory[*ottlspanevent.TransformContext], errorMode ottl.ErrorMode, set component.TelemetrySettings, sequenceOptions ...ottlspanevent.ConditionSequenceOption) (*ottl.ConditionSequence[*ottlspanevent.TransformContext], error) {
	parser, err := ottlspanevent.NewParser(functions, set, ottlspanevent.EnablePathContextNames())
	if err != nil {
		return nil, err
	}
	return newBoolExprWithPathContextNames(ottlspanevent.ContextName, parser, conditions, set, func(parsed []*ottl.Condition[*ottlspanevent.TransformContext]) ottl.ConditionSequence[*ottlspanevent.TransformContext] {
		return ottlspanevent.NewConditionSequence(parsed, set, slices.Concat(sequenceOptions, []ottlspanevent.ConditionSequenceOption{ottlspanevent.WithConditionSequenceErrorMode(errorMode)})...)
	})
}

// NewBoolExprForMetricWithPathContextNames is like NewBoolExprForMetric, but conditions may use OTTL
// path context names (e.g. `metric.name`). Conditions without an explicit context are rewritten to
// use the metric context.
func NewBoolExprForMetricWithPathContextNames(conditions []string, functions map[string]ottl.Factory[*ottlmetric.TransformContext], errorMode ottl.ErrorMode, set component.TelemetrySettings, sequenceOptions ...ottlmetric.ConditionSequenceOption) (*ottl.ConditionSequence[*ottlmetric.TransformContext], error) {
	parser, err := ottlmetric.NewParser(functions, set, ottlmetric.EnablePathContextNames())
	if err != nil {
		return nil, err
	}
	return newBoolExprWithPathContextNames(ottlmetric.ContextName, parser, conditions, set, func(parsed []*ottl.Condition[*ottlmetric.TransformContext]) ottl.ConditionSequence[*ottlmetric.TransformContext] {
		return ottlmetric.NewConditionSequence(parsed, set, slices.Concat(sequenceOptions, []ottlmetric.ConditionSequenceOption{ottlmetric.WithConditionSequenceErrorMode(errorMode)})...)
	})
}

// NewBoolExprForDataPointWithPathContextNames is like NewBoolExprForDataPoint, but conditions may
// use OTTL path context names (e.g. `datapoint.attributes["foo"]`). Conditions without an explicit
// context are rewritten to use the datapoint context.
func NewBoolExprForDataPointWithPathContextNames(conditions []string, functions map[string]ottl.Factory[*ottldatapoint.TransformContext], errorMode ottl.ErrorMode, set component.TelemetrySettings, sequenceOptions ...ottldatapoint.ConditionSequenceOption) (*ottl.ConditionSequence[*ottldatapoint.TransformContext], error) {
	parser, err := ottldatapoint.NewParser(functions, set, ottldatapoint.EnablePathContextNames())
	if err != nil {
		return nil, err
	}
	return newBoolExprWithPathContextNames(ottldatapoint.ContextName, parser, conditions, set, func(parsed []*ottl.Condition[*ottldatapoint.TransformContext]) ottl.ConditionSequence[*ottldatapoint.TransformContext] {
		return ottldatapoint.NewConditionSequence(parsed, set, slices.Concat(sequenceOptions, []ottldatapoint.ConditionSequenceOption{ottldatapoint.WithConditionSequenceErrorMode(errorMode)})...)
	})
}

// NewBoolExprForLogWithPathContextNames is like NewBoolExprForLog, but conditions may use OTTL path
// context names (e.g. `log.attributes["foo"]`). Conditions without an explicit context are
// rewritten to use the log context.
func NewBoolExprForLogWithPathContextNames(conditions []string, functions map[string]ottl.Factory[*ottllog.TransformContext], errorMode ottl.ErrorMode, set component.TelemetrySettings, sequenceOptions ...ottllog.ConditionSequenceOption) (*ottl.ConditionSequence[*ottllog.TransformContext], error) {
	parser, err := ottllog.NewParser(functions, set, ottllog.EnablePathContextNames())
	if err != nil {
		return nil, err
	}
	return newBoolExprWithPathContextNames(ottllog.ContextName, parser, conditions, set, func(parsed []*ottl.Condition[*ottllog.TransformContext]) ottl.ConditionSequence[*ottllog.TransformContext] {
		return ottllog.NewConditionSequence(parsed, set, slices.Concat(sequenceOptions, []ottllog.ConditionSequenceOption{ottllog.WithConditionSequenceErrorMode(errorMode)})...)
	})
}

// NewBoolExprForProfileWithPathContextNames is like NewBoolExprForProfile, but conditions may use
// OTTL path context names (e.g. `profile.attributes["foo"]`). Conditions without an explicit
// context are rewritten to use the profile context.
func NewBoolExprForProfileWithPathContextNames(conditions []string, functions map[string]ottl.Factory[*ottlprofile.TransformContext], errorMode ottl.ErrorMode, set component.TelemetrySettings, sequenceOptions ...ottlprofile.ConditionSequenceOption) (*ottl.ConditionSequence[*ottlprofile.TransformContext], error) {
	parser, err := ottlprofile.NewParser(functions, set, ottlprofile.EnablePathContextNames())
	if err != nil {
		return nil, err
	}
	return newBoolExprWithPathContextNames(ottlprofile.ContextName, parser, conditions, set, func(parsed []*ottl.Condition[*ottlprofile.TransformContext]) ottl.ConditionSequence[*ottlprofile.TransformContext] {
		return ottlprofile.NewConditionSequence(parsed, set, slices.Concat(sequenceOptions, []ottlprofile.ConditionSequenceOption{ottlprofile.WithConditionSequenceErrorMode(errorMode)})...)
	})
}

// NewBoolExprForResourceWithPathContextNames is like NewBoolExprForResource, but conditions may use
// OTTL path context names (e.g. `resource.attributes["foo"]`). Conditions without an explicit
// context are rewritten to use the resource context.
func NewBoolExprForResourceWithPathContextNames(conditions []string, functions map[string]ottl.Factory[*ottlresource.TransformContext], errorMode ottl.ErrorMode, set component.TelemetrySettings, sequenceOptions ...ottlresource.ConditionSequenceOption) (*ottl.ConditionSequence[*ottlresource.TransformContext], error) {
	parser, err := ottlresource.NewParser(functions, set, ottlresource.EnablePathContextNames())
	if err != nil {
		return nil, err
	}
	return newBoolExprWithPathContextNames(ottlresource.ContextName, parser, conditions, set, func(parsed []*ottl.Condition[*ottlresource.TransformContext]) ottl.ConditionSequence[*ottlresource.TransformContext] {
		return ottlresource.NewConditionSequence(parsed, set, slices.Concat(sequenceOptions, []ottlresource.ConditionSequenceOption{ottlresource.WithConditionSequenceErrorMode(errorMode)})...)
	})
}

// NewBoolExprForScopeWithPathContextNames is like NewBoolExprForScope, but conditions may use OTTL
// path context names (e.g. `scope.name`). Conditions without an explicit context are rewritten to
// use the scope context.
func NewBoolExprForScopeWithPathContextNames(conditions []string, functions map[string]ottl.Factory[*ottlscope.TransformContext], errorMode ottl.ErrorMode, set component.TelemetrySettings, sequenceOptions ...ottlscope.ConditionSequenceOption) (*ottl.ConditionSequence[*ottlscope.TransformContext], error) {
	parser, err := ottlscope.NewParser(functions, set, ottlscope.EnablePathContextNames())
	if err != nil {
		return nil, err
	}
	return newBoolExprWithPathContextNames(ottlscope.ContextName, parser, conditions, set, func(parsed []*ottl.Condition[*ottlscope.TransformContext]) ottl.ConditionSequence[*ottlscope.TransformContext] {
		return ottlscope.NewConditionSequence(parsed, set, slices.Concat(sequenceOptions, []ottlscope.ConditionSequenceOption{ottlscope.WithConditionSequenceErrorMode(errorMode)})...)
	})
}
