// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package condition // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor/internal/condition"

import (
	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlresource"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlscope"
)

// CommonConditions is a generic interface for signal-specific condition types
// (LogConditions, MetricConditions, TraceConditions, ProfileConditions).
// It provides factory methods to create signal-specific conditions from
// resource and scope level OTTL conditions
type CommonConditions[R any] interface {
	newFromResource(rc []*ottl.Condition[*ottlresource.TransformContext], telemetrySettings component.TelemetrySettings, errorMode ottl.ErrorMode) R
	newFromScope(sc []*ottl.Condition[*ottlscope.TransformContext], telemetrySettings component.TelemetrySettings, errorMode ottl.ErrorMode) R
}

func withCommonParsers[R CommonConditions[R]](resourceFunctions map[string]ottl.Factory[*ottlresource.TransformContext]) ottl.ParserCollectionOption[R] {
	return func(pc *ottl.ParserCollection[R]) error {
		rp, err := ottlresource.NewParser(resourceFunctions, pc.Settings, ottlresource.EnablePathContextNames())
		if err != nil {
			return err
		}
		sp, err := ottlscope.NewParser(filterottl.StandardScopeFuncs(), pc.Settings, ottlscope.EnablePathContextNames())
		if err != nil {
			return err
		}

		err = ottl.WithParserCollectionContext(ottlresource.ContextName, &rp, ottl.WithConditionConverter[*ottlresource.TransformContext, R](convertResourceConditions))(pc)
		if err != nil {
			return err
		}

		err = ottl.WithParserCollectionContext(ottlscope.ContextName, &sp, ottl.WithConditionConverter[*ottlscope.TransformContext, R](convertScopeConditions))(pc)
		if err != nil {
			return err
		}

		return nil
	}
}

func convertResourceConditions[R CommonConditions[R]](
	pc *ottl.ParserCollection[R],
	conditions ottl.ConditionsGetter,
	parsedConditions []*ottl.Condition[*ottlresource.TransformContext],
) (R, error) {
	contextConditions, err := toContextConditions(conditions)
	if err != nil {
		return *new(R), err
	}
	errorMode := getErrorMode(pc, contextConditions)
	var r R
	return r.newFromResource(parsedConditions, pc.Settings, errorMode), nil
}

func convertScopeConditions[R CommonConditions[R]](
	pc *ottl.ParserCollection[R],
	conditions ottl.ConditionsGetter,
	parsedConditions []*ottl.Condition[*ottlscope.TransformContext],
) (R, error) {
	contextConditions, err := toContextConditions(conditions)
	if err != nil {
		return *new(R), err
	}
	errorMode := getErrorMode(pc, contextConditions)
	var r R
	return r.newFromScope(parsedConditions, pc.Settings, errorMode), nil
}
