// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package contextfilter // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor/internal/contextfilter"

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
)

type ProfilesConsumer interface {
	Context() ContextID
	ConsumeProfiles(ctx context.Context, ld pprofile.Profiles) error
}

type profileConditions struct {
	expr.BoolExpr[ottlprofile.TransformContext]
}

func (profileConditions) Context() ContextID {
	return Profile
}

func (l profileConditions) ConsumeProfiles(ctx context.Context, pd pprofile.Profiles) error {
	var condErr error
	pd.ResourceProfiles().RemoveIf(func(rprofiles pprofile.ResourceProfiles) bool {
		rprofiles.ScopeProfiles().RemoveIf(func(sprofiles pprofile.ScopeProfiles) bool {
			sprofiles.Profiles().RemoveIf(func(profile pprofile.Profile) bool {
				tCtx := ottlprofile.NewTransformContext(profile, pd.Dictionary(), sprofiles.Scope(), rprofiles.Resource(), sprofiles, rprofiles)
				cond, err := l.Eval(ctx, tCtx)
				if err != nil {
					condErr = multierr.Append(condErr, err)
					return false
				}
				return cond
			})
			return sprofiles.Profiles().Len() == 0
		})
		return rprofiles.ScopeProfiles().Len() == 0
	})

	if pd.ResourceProfiles().Len() == 0 {
		return processorhelper.ErrSkipProcessingData
	}
	return condErr
}

type ProfileParserCollection ottl.ParserCollection[ProfilesConsumer]

type ProfileParserCollectionOption ottl.ParserCollectionOption[ProfilesConsumer]

func WithProfileParser(functions map[string]ottl.Factory[ottlprofile.TransformContext]) ProfileParserCollectionOption {
	return func(pc *ottl.ParserCollection[ProfilesConsumer]) error {
		profileParser, err := ottlprofile.NewParser(functions, pc.Settings, ottlprofile.EnablePathContextNames())
		if err != nil {
			return err
		}
		return ottl.WithParserCollectionContext(ottlprofile.ContextName, &profileParser, ottl.WithConditionConverter(convertProfileConditions))(pc)
	}
}

func WithProfileErrorMode(errorMode ottl.ErrorMode) ProfileParserCollectionOption {
	return ProfileParserCollectionOption(ottl.WithParserCollectionErrorMode[ProfilesConsumer](errorMode))
}

func WithProfileCommonParsers(functions map[string]ottl.Factory[*ottlresource.TransformContext]) ProfileParserCollectionOption {
	return ProfileParserCollectionOption(withCommonParsers[ProfilesConsumer](functions))
}

func NewProfileParserCollection(settings component.TelemetrySettings, options ...ProfileParserCollectionOption) (*ProfileParserCollection, error) {
	pcOptions := []ottl.ParserCollectionOption[ProfilesConsumer]{
		ottl.EnableParserCollectionModifiedPathsLogging[ProfilesConsumer](true),
	}

	for _, option := range options {
		pcOptions = append(pcOptions, ottl.ParserCollectionOption[ProfilesConsumer](option))
	}

	pc, err := ottl.NewParserCollection(settings, pcOptions...)
	if err != nil {
		return nil, err
	}

	ppc := ProfileParserCollection(*pc)
	return &ppc, nil
}

func convertProfileConditions(pc *ottl.ParserCollection[ProfilesConsumer], conditions ottl.ConditionsGetter, parsedConditions []*ottl.Condition[ottlprofile.TransformContext]) (ProfilesConsumer, error) {
	contextConditions, err := toContextConditions(conditions)
	if err != nil {
		return nil, err
	}

	errorMode := getErrorMode(pc, contextConditions)
	lConditions := ottlprofile.NewConditionSequence(parsedConditions, pc.Settings, ottlprofile.WithConditionSequenceErrorMode(errorMode))
	return profileConditions{&lConditions}, nil
}

func (ppc *ProfileParserCollection) ParseContextConditions(contextConditions ContextConditions) (ProfilesConsumer, error) {
	pc := ottl.ParserCollection[ProfilesConsumer](*ppc)
	if contextConditions.Context != "" {
		return pc.ParseConditionsWithContext(string(contextConditions.Context), contextConditions, true)
	}
	return pc.ParseConditions(contextConditions)
}
