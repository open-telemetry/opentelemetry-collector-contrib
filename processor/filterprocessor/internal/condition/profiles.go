// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package condition // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor/internal/condition"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.opentelemetry.io/collector/processor/processorhelper"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/expr"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlprofile"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlresource"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlscope"
)

type ProfilesConsumer struct {
	ResourceExpr expr.BoolExpr[*ottlresource.TransformContext]
	ScopeExpr    expr.BoolExpr[*ottlscope.TransformContext]
	ProfileExpr  expr.BoolExpr[ottlprofile.TransformContext]
}

// ProfileConditions is the type R for ParserCollection[R] that holds parsed OTTL conditions
type ProfileConditions struct {
	resourceConditions []*ottl.Condition[*ottlresource.TransformContext]
	scopeConditions    []*ottl.Condition[*ottlscope.TransformContext]
	profileConditions  []*ottl.Condition[ottlprofile.TransformContext]
	telemetrySettings  component.TelemetrySettings
	errorMode          ottl.ErrorMode
}

func (pc ProfilesConsumer) ConsumeProfiles(ctx context.Context, pd pprofile.Profiles) error {
	var condErr error
	pd.ResourceProfiles().RemoveIf(func(rp pprofile.ResourceProfiles) bool {
		if pc.ResourceExpr != nil {
			rCtx := ottlresource.NewTransformContextPtr(rp.Resource(), rp)
			rCond, err := pc.ResourceExpr.Eval(ctx, rCtx)
			rCtx.Close()
			if err != nil {
				condErr = multierr.Append(condErr, err)
				return false
			}
			if rCond {
				return true
			}
		}

		if pc.ScopeExpr == nil && pc.ProfileExpr == nil {
			return rp.ScopeProfiles().Len() == 0
		}

		rp.ScopeProfiles().RemoveIf(func(sp pprofile.ScopeProfiles) bool {
			if pc.ScopeExpr != nil {
				sCtx := ottlscope.NewTransformContextPtr(sp.Scope(), rp.Resource(), sp)
				sCond, err := pc.ScopeExpr.Eval(ctx, sCtx)
				sCtx.Close()
				if err != nil {
					condErr = multierr.Append(condErr, err)
					return false
				}
				if sCond {
					return true
				}
			}

			if pc.ProfileExpr != nil {
				sp.Profiles().RemoveIf(func(profile pprofile.Profile) bool {
					tCtx := ottlprofile.NewTransformContext(profile, pd.Dictionary(), sp.Scope(), rp.Resource(), sp, rp)
					cond, err := pc.ProfileExpr.Eval(ctx, tCtx)
					if err != nil {
						condErr = multierr.Append(condErr, err)
						return false
					}
					return cond
				})
			}
			return sp.Profiles().Len() == 0
		})
		return rp.ScopeProfiles().Len() == 0
	})

	if pd.ResourceProfiles().Len() == 0 {
		return processorhelper.ErrSkipProcessingData
	}
	return condErr
}

func (ProfileConditions) newFromResource(rc []*ottl.Condition[*ottlresource.TransformContext], telemetrySettings component.TelemetrySettings, errorMode ottl.ErrorMode) ProfileConditions {
	return ProfileConditions{
		resourceConditions: rc,
		telemetrySettings:  telemetrySettings,
		errorMode:          errorMode,
	}
}

func (ProfileConditions) newFromScope(sc []*ottl.Condition[*ottlscope.TransformContext], telemetrySettings component.TelemetrySettings, errorMode ottl.ErrorMode) ProfileConditions {
	return ProfileConditions{
		scopeConditions:   sc,
		telemetrySettings: telemetrySettings,
		errorMode:         errorMode,
	}
}

func (pc ProfileConditions) ToProfilesConsumer() ProfilesConsumer {
	var rExpr expr.BoolExpr[*ottlresource.TransformContext]
	var sExpr expr.BoolExpr[*ottlscope.TransformContext]
	var pExpr expr.BoolExpr[ottlprofile.TransformContext]

	if len(pc.resourceConditions) > 0 {
		cs := ottlresource.NewConditionSequence(pc.resourceConditions, pc.telemetrySettings, ottlresource.WithConditionSequenceErrorMode(pc.errorMode))
		rExpr = &cs
	}

	if len(pc.scopeConditions) > 0 {
		cs := ottlscope.NewConditionSequence(pc.scopeConditions, pc.telemetrySettings, ottlscope.WithConditionSequenceErrorMode(pc.errorMode))
		sExpr = &cs
	}

	if len(pc.profileConditions) > 0 {
		cs := ottlprofile.NewConditionSequence(pc.profileConditions, pc.telemetrySettings, ottlprofile.WithConditionSequenceErrorMode(pc.errorMode))
		pExpr = &cs
	}

	return ProfilesConsumer{
		ResourceExpr: rExpr,
		ScopeExpr:    sExpr,
		ProfileExpr:  pExpr,
	}
}

type ProfileParserCollection ottl.ParserCollection[ProfileConditions]

type ProfileParserCollectionOption ottl.ParserCollectionOption[ProfileConditions]

func WithProfileParser(functions map[string]ottl.Factory[ottlprofile.TransformContext]) ProfileParserCollectionOption {
	return func(pc *ottl.ParserCollection[ProfileConditions]) error {
		profileParser, err := ottlprofile.NewParser(functions, pc.Settings, ottlprofile.EnablePathContextNames())
		if err != nil {
			return err
		}
		return ottl.WithParserCollectionContext(ottlprofile.ContextName, &profileParser, ottl.WithConditionConverter(convertProfileConditions))(pc)
	}
}

func WithProfileErrorMode(errorMode ottl.ErrorMode) ProfileParserCollectionOption {
	return ProfileParserCollectionOption(ottl.WithParserCollectionErrorMode[ProfileConditions](errorMode))
}

func WithProfileCommonParsers(functions map[string]ottl.Factory[*ottlresource.TransformContext]) ProfileParserCollectionOption {
	return ProfileParserCollectionOption(withCommonParsers[ProfileConditions](functions))
}

func NewProfileParserCollection(settings component.TelemetrySettings, options ...ProfileParserCollectionOption) (*ProfileParserCollection, error) {
	pcOptions := []ottl.ParserCollectionOption[ProfileConditions]{
		ottl.EnableParserCollectionModifiedPathsLogging[ProfileConditions](true),
	}

	for _, option := range options {
		pcOptions = append(pcOptions, ottl.ParserCollectionOption[ProfileConditions](option))
	}

	pc, err := ottl.NewParserCollection(settings, pcOptions...)
	if err != nil {
		return nil, err
	}

	ppc := ProfileParserCollection(*pc)
	return &ppc, nil
}

func convertProfileConditions(pc *ottl.ParserCollection[ProfileConditions], conditions ottl.ConditionsGetter, parsedConditions []*ottl.Condition[ottlprofile.TransformContext]) (ProfileConditions, error) {
	contextConditions, err := toContextConditions(conditions)
	if err != nil {
		return ProfileConditions{}, err
	}

	errorMode := getErrorMode(pc, contextConditions)
	return ProfileConditions{
		profileConditions: parsedConditions,
		telemetrySettings: pc.Settings,
		errorMode:         errorMode,
	}, nil
}

func (ppc *ProfileParserCollection) ParseContextConditions(contextConditions ContextConditions) (ProfileConditions, error) {
	pc := ottl.ParserCollection[ProfileConditions](*ppc)
	if contextConditions.Context != "" {
		pConditions, err := pc.ParseConditionsWithContext(string(contextConditions.Context), contextConditions, true)
		if err != nil {
			return ProfileConditions{}, err
		}
		return pConditions, nil
	}

	var rConditions []*ottl.Condition[*ottlresource.TransformContext]
	var sConditions []*ottl.Condition[*ottlscope.TransformContext]
	var pConditions []*ottl.Condition[ottlprofile.TransformContext]

	for _, cc := range contextConditions.Conditions {
		profConditions, err := pc.ParseConditions(ContextConditions{Conditions: []string{cc}})
		if err != nil {
			return ProfileConditions{}, err
		}

		if len(profConditions.resourceConditions) > 0 {
			rConditions = append(rConditions, profConditions.resourceConditions...)
		}
		if len(profConditions.scopeConditions) > 0 {
			sConditions = append(sConditions, profConditions.scopeConditions...)
		}
		if len(profConditions.profileConditions) > 0 {
			pConditions = append(pConditions, profConditions.profileConditions...)
		}
	}

	aggregatedConditions := ProfileConditions{
		resourceConditions: rConditions,
		scopeConditions:    sConditions,
		profileConditions:  pConditions,
		telemetrySettings:  pc.Settings,
		errorMode:          pc.ErrorMode,
	}

	return aggregatedConditions, nil
}
