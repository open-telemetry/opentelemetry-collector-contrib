// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package attributesprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/attributesprocessor"

import (
	"context"

	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/attraction"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/expr"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
)

type logAttributesProcessor struct {
	logger   *zap.Logger
	attrProc *attraction.AttrProc
	skipExpr expr.BoolExpr[ottllog.TransformContext]
}

// newLogAttributesProcessor returns a processor that modifies attributes of a
// log record. To construct the attributes processors, the use of the factory
// methods are required in order to validate the inputs.
func newLogAttributesProcessor(logger *zap.Logger, attrProc *attraction.AttrProc, skipExpr expr.BoolExpr[ottllog.TransformContext]) *logAttributesProcessor {
	return &logAttributesProcessor{
		logger:   logger,
		attrProc: attrProc,
		skipExpr: skipExpr,
	}
}

func (a *logAttributesProcessor) processLogs(ctx context.Context, ld plog.Logs) (plog.Logs, error) {
	rls := ld.ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		rs := rls.At(i)
		ilss := rs.ScopeLogs()
		resource := rs.Resource()
		for j := 0; j < ilss.Len(); j++ {
			ils := ilss.At(j)
			logs := ils.LogRecords()
			library := ils.Scope()
			for k := 0; k < logs.Len(); k++ {
				lr := logs.At(k)
				if a.skipExpr != nil {
					skip, err := a.skipExpr.Eval(ctx, ottllog.NewTransformContext(lr, library, resource))
					if err != nil {
						return ld, err
					}
					if skip {
						continue
					}
				}

				a.attrProc.Process(ctx, a.logger, lr.Attributes())
			}
		}
	}
	return ld, nil
}
