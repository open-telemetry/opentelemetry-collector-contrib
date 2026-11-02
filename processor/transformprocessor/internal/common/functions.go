// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package common // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"

import (
	"fmt"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlresource"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlscope"
	xprofilefuncs "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/xprofile/ottlfuncs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
	xottlfuncs "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/xottl/ottlfuncs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/metadata"
)

func ResourceFunctions() map[string]ottl.Factory[*ottlresource.TransformContext] {
	return xottlfuncs.WithExperimentalConverters(xprofilefuncs.WithProfileConverters(ottlfuncs.StandardFuncs[*ottlresource.TransformContext]()))
}

func ScopeFunctions() map[string]ottl.Factory[*ottlscope.TransformContext] {
	return xottlfuncs.WithExperimentalConverters(xprofilefuncs.WithProfileConverters(ottlfuncs.StandardFuncs[*ottlscope.TransformContext]()))
}

// NewExperimentalFactory rejects f unless the processor.transform.enableExperimentalFunctions feature gate is enabled.
func NewExperimentalFactory[K any](f ottl.Factory[K]) ottl.Factory[K] {
	return ottl.NewFactory(f.Name(), f.CreateDefaultArguments(), func(fCtx ottl.FunctionContext, args ottl.Arguments) (ottl.ExprFunc[K], error) {
		gate := metadata.ProcessorTransformEnableExperimentalFunctionsFeatureGate
		if !gate.IsEnabled() {
			return nil, fmt.Errorf("function %q is experimental and requires the `%s` feature gate to be enabled", f.Name(), gate.ID())
		}
		fCtx.Set.Logger.Warn(
			"transform processor configuration uses an experimental function that is not covered by stability guarantees; its name, arguments, and behavior may change or be removed",
			zap.String("function", f.Name()),
		)
		return f.CreateFunction(fCtx, args)
	})
}
