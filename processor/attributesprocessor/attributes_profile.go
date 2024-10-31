// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package attributesprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/attributesprocessor"

import (
	"context"

	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/attraction"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/expr"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlprofile"
)

type profileAttributesProcessor struct {
	profileger *zap.Logger
	attrProc   *attraction.AttrProc
	skipExpr   expr.BoolExpr[ottlprofile.TransformContext]
}

// newProfileAttributesProcessor returns a processor that modifies attributes of a
// profile record. To construct the attributes processors, the use of the factory
// methods are required in order to validate the inputs.
func newProfileAttributesProcessor(profileger *zap.Logger, attrProc *attraction.AttrProc, skipExpr expr.BoolExpr[ottlprofile.TransformContext]) *profileAttributesProcessor {
	return &profileAttributesProcessor{
		profileger: profileger,
		attrProc:   attrProc,
		skipExpr:   skipExpr,
	}
}

func (a *profileAttributesProcessor) processProfiles(ctx context.Context, ld pprofile.Profiles) (pprofile.Profiles, error) {
	rls := ld.ResourceProfiles()
	for i := 0; i < rls.Len(); i++ {
		rs := rls.At(i)
		ilss := rs.ScopeProfiles()
		resource := rs.Resource()
		for j := 0; j < ilss.Len(); j++ {
			ils := ilss.At(j)
			profiles := ils.Profiles()
			library := ils.Scope()
			for k := 0; k < profiles.Len(); k++ {
				lr := profiles.At(k)
				if a.skipExpr != nil {
					skip, err := a.skipExpr.Eval(ctx, ottlprofile.NewTransformContext(lr, library, resource, ils, rs))
					if err != nil {
						return ld, err
					}
					if skip {
						continue
					}
				}

				a.attrProc.Process(ctx, a.profileger, lr.Attributes())
			}
		}
	}
	return ld, nil
}
