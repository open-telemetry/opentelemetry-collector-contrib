// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package attributesprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/attributesprocessor"

import (
	"context"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/attraction"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/expr"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlprofile"
)

type profileAttributesProcessor struct {
	logger   *zap.Logger
	attrProc *attraction.AttrProc
	skipExpr expr.BoolExpr[ottlprofile.TransformContext]
}

// newProfileAttributesProcessor returns a processor that modifies attributes of a
// profile. To construct the attributes processors, the use of the factory
// methods is required in order to validate the inputs.
func newProfileAttributesProcessor(logger *zap.Logger, attrProc *attraction.AttrProc, skipExpr expr.BoolExpr[ottlprofile.TransformContext]) *profileAttributesProcessor {
	return &profileAttributesProcessor{
		logger:   logger,
		attrProc: attrProc,
		skipExpr: skipExpr,
	}
}

func (a *profileAttributesProcessor) processProfiles(ctx context.Context, pd pprofile.Profiles) (pprofile.Profiles, error) {
	rps := pd.ResourceProfiles()
	for i := 0; i < rps.Len(); i++ {
		rs := rps.At(i)
		ilps := rs.ScopeProfiles()
		resource := rs.Resource()
		for j := 0; j < ilps.Len(); j++ {
			ils := ilps.At(j)
			profiles := ils.Profiles()
			library := ils.Scope()
			for k := 0; k < profiles.Len(); k++ {
				p := profiles.At(k)
				if a.skipExpr != nil {
					skip, err := a.skipExpr.Eval(ctx, ottlprofile.NewTransformContext(p, library, resource, ils, rs))
					if err != nil {
						return pd, err
					}
					if skip {
						continue
					}
				}

				var err error
				m := pprofile.FromAttributeIndices(p.AttributeTable(), p)
				a.attrProc.Process(ctx, a.logger, m)
				m.Range(func(k string, v pcommon.Value) bool {
					err = pprofile.AddAttribute(p.AttributeTable(), p, k, v)
					return err == nil
				})
				if err != nil {
					return pd, err
				}
			}
		}
	}
	return pd, nil
}
