// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package traces // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/traces"

import (
	"maps"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspanevent"
	xprofilefuncs "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/xprofile/ottlfuncs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
)

func SpanFunctions() map[string]ottl.Factory[*ottlspan.TransformContext] {
	functions := xprofilefuncs.WithProfileConverters(ottlfuncs.StandardFuncs[*ottlspan.TransformContext]())

	spanFunctions := ottl.CreateFactoryMap(
		ottlfuncs.NewIsRootSpanFactory(),
		NewSetSemconvSpanNameFactory(),
	)

	maps.Copy(functions, spanFunctions)

	return functions
}

func SpanEventFunctions() map[string]ottl.Factory[*ottlspanevent.TransformContext] {
	// No trace-only functions yet.
	return xprofilefuncs.WithProfileConverters(ottlfuncs.StandardFuncs[*ottlspanevent.TransformContext]())
}
