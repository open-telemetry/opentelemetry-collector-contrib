// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package common // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pprofile"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/expr"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlprofile"
)

type ProfilesConsumer interface {
	Context() ContextID
	ConsumeProfiles(ctx context.Context, ld pprofile.Profiles) error
}

type profileStatements struct {
	ottl.StatementSequence[ottlprofile.TransformContext]
	expr.BoolExpr[ottlprofile.TransformContext]
}

func (profileStatements) Context() ContextID {
	return Profile
}

func (l profileStatements) ConsumeProfiles(ctx context.Context, ld pprofile.Profiles) error {
	dic := ld.Dictionary()
	for _, rprofiles := range ld.ResourceProfiles().All() {
		for _, sprofiles := range rprofiles.ScopeProfiles().All() {
			for _, profile := range sprofiles.Profiles().All() {
				tCtx := ottlprofile.NewTransformContext(profile, dic, sprofiles.Scope(), rprofiles.Resource(), sprofiles, rprofiles)
				condition, err := l.Eval(ctx, tCtx)
				if err != nil {
					return err
				}
				if condition {
					err := l.Execute(ctx, tCtx)
					if err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

type ProfileParserCollection ottl.ParserCollection[ProfilesConsumer]

type ProfileParserCollectionOption ottl.ParserCollectionOption[ProfilesConsumer]

func WithProfileParser(functions map[string]ottl.Factory[ottlprofile.TransformContext]) ProfileParserCollectionOption {
	return func(pc *ottl.ParserCollection[ProfilesConsumer]) error {
		profileParser, err := ottlprofile.NewParser(functions, pc.Settings, ottlprofile.EnablePathContextNames())
		if err != nil {
			return err
		}
		return ottl.WithParserCollectionContext(ottlprofile.ContextName, &profileParser, ottl.WithStatementConverter(convertProfileStatements))(pc)
	}
}

func WithProfileErrorMode(errorMode ottl.ErrorMode) ProfileParserCollectionOption {
	return ProfileParserCollectionOption(ottl.WithParserCollectionErrorMode[ProfilesConsumer](errorMode))
}

func NewProfileParserCollection(settings component.TelemetrySettings, options ...ProfileParserCollectionOption) (*ProfileParserCollection, error) {
	pcOptions := []ottl.ParserCollectionOption[ProfilesConsumer]{
		withCommonContextParsers[ProfilesConsumer](),
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

func convertProfileStatements(pc *ottl.ParserCollection[ProfilesConsumer], statements ottl.StatementsGetter, parsedStatements []*ottl.Statement[ottlprofile.TransformContext]) (ProfilesConsumer, error) {
	contextStatements, err := toContextStatements(statements)
	if err != nil {
		return nil, err
	}
	errorMode := pc.ErrorMode
	if contextStatements.ErrorMode != "" {
		errorMode = contextStatements.ErrorMode
	}
	var parserOptions []ottl.Option[ottlprofile.TransformContext]
	if contextStatements.Context == "" {
		parserOptions = append(parserOptions, ottlprofile.EnablePathContextNames())
	}
	globalExpr, errGlobalBoolExpr := parseGlobalExpr(filterottl.NewBoolExprForProfileWithOptions, contextStatements.Conditions, errorMode, pc.Settings, filterottl.StandardProfileFuncs(), parserOptions)
	if errGlobalBoolExpr != nil {
		return nil, errGlobalBoolExpr
	}
	lStatements := ottlprofile.NewStatementSequence(parsedStatements, pc.Settings, ottlprofile.WithStatementSequenceErrorMode(errorMode))
	return profileStatements{lStatements, globalExpr}, nil
}

func (ppc *ProfileParserCollection) ParseContextStatements(contextStatements ContextStatements) (ProfilesConsumer, error) {
	pc := ottl.ParserCollection[ProfilesConsumer](*ppc)
	if contextStatements.Context != "" {
		return pc.ParseStatementsWithContext(string(contextStatements.Context), contextStatements, true)
	}
	return pc.ParseStatements(contextStatements, ottl.WithContextInferenceConditions(contextStatements.Conditions))
}
