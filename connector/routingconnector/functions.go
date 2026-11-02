// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package routingconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector"

import (
	"context"
	"maps"
	"slices"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottldatapoint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlotelcol"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlresource"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
	xprofilefuncs "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/xprofile/ottlfuncs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
	xottlfuncs "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/xottl/ottlfuncs"
)

const routeFunctionName = "route"

func createRouteFunction[K any](ottl.FunctionContext, ottl.Arguments) (ottl.ExprFunc[K], error) {
	return func(context.Context, K) (any, error) {
		return true, nil
	}, nil
}

func routeFunctionFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory(routeFunctionName, nil, createRouteFunction[K])
}

func standardFunctions[K any]() map[string]ottl.Factory[K] {
	// standard converters do not transform data, so we can safely use them
	funcs := xottlfuncs.WithExperimentalConverters(xprofilefuncs.WithProfileConverters(ottlfuncs.StandardConverters[K]()))

	deleteKey := ottlfuncs.NewDeleteKeyFactory[K]()
	funcs[deleteKey.Name()] = deleteKey

	deleteMatchingKeys := ottlfuncs.NewDeleteMatchingKeysFactory[K]()
	funcs[deleteMatchingKeys.Name()] = deleteMatchingKeys

	route := routeFunctionFactory[K]()
	funcs[route.Name()] = route

	return funcs
}

func spanFunctions() map[string]ottl.Factory[*ottlspan.TransformContext] {
	funcs := standardFunctions[*ottlspan.TransformContext]()

	isRootSpan := ottlfuncs.NewIsRootSpanFactory()
	funcs[isRootSpan.Name()] = isRootSpan

	return funcs
}

func defaultOtelColFunctionsMap() map[string]ottl.Factory[*ottlotelcol.TransformContext] {
	return standardFunctions[*ottlotelcol.TransformContext]()
}

func defaultResourceFunctionsMap() map[string]ottl.Factory[*ottlresource.TransformContext] {
	return standardFunctions[*ottlresource.TransformContext]()
}

func defaultSpanFunctionsMap() map[string]ottl.Factory[*ottlspan.TransformContext] {
	return spanFunctions()
}

func defaultMetricFunctionsMap() map[string]ottl.Factory[*ottlmetric.TransformContext] {
	return standardFunctions[*ottlmetric.TransformContext]()
}

func defaultDataPointFunctionsMap() map[string]ottl.Factory[*ottldatapoint.TransformContext] {
	return standardFunctions[*ottldatapoint.TransformContext]()
}

func defaultLogFunctionsMap() map[string]ottl.Factory[*ottllog.TransformContext] {
	return standardFunctions[*ottllog.TransformContext]()
}

// DefaultOtelColFunctions returns the default set of OTTL functions available to routing table
// entries using the "otelcol" context.
func DefaultOtelColFunctions() []ottl.Factory[*ottlotelcol.TransformContext] {
	return slices.Collect(maps.Values(defaultOtelColFunctionsMap()))
}

// DefaultResourceFunctions returns the default set of OTTL functions available to routing table
// entries using the "resource" context.
func DefaultResourceFunctions() []ottl.Factory[*ottlresource.TransformContext] {
	return slices.Collect(maps.Values(defaultResourceFunctionsMap()))
}

// DefaultSpanFunctions returns the default set of OTTL functions available to routing table
// entries using the "span" context.
func DefaultSpanFunctions() []ottl.Factory[*ottlspan.TransformContext] {
	return slices.Collect(maps.Values(defaultSpanFunctionsMap()))
}

// DefaultMetricFunctions returns the default set of OTTL functions available to routing table
// entries using the "metric" context.
func DefaultMetricFunctions() []ottl.Factory[*ottlmetric.TransformContext] {
	return slices.Collect(maps.Values(defaultMetricFunctionsMap()))
}

// DefaultDataPointFunctions returns the default set of OTTL functions available to routing table
// entries using the "datapoint" context.
func DefaultDataPointFunctions() []ottl.Factory[*ottldatapoint.TransformContext] {
	return slices.Collect(maps.Values(defaultDataPointFunctionsMap()))
}

// DefaultLogFunctions returns the default set of OTTL functions available to routing table
// entries using the "log" context.
func DefaultLogFunctions() []ottl.Factory[*ottllog.TransformContext] {
	return slices.Collect(maps.Values(defaultLogFunctionsMap()))
}

func mergeFunctionsToMap[K any](functionMap map[string]ottl.Factory[K], functions []ottl.Factory[K]) map[string]ottl.Factory[K] {
	for _, f := range functions {
		functionMap[f.Name()] = f
	}
	return functionMap
}
