// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package transformprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor"

import (
	"maps"
	"slices"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottldatapoint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlprofile"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspanevent"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/logs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/metrics"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/profiles"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/traces"
)

// Deprecated: [v0.142.0] use DefaultLogFunctionsNew.
func DefaultLogFunctions() []ottl.Factory[ottllog.TransformContext] {
	return slices.Collect(maps.Values(ottlfuncs.StandardConverters[ottllog.TransformContext]()))
}

func DefaultLogFunctionsNew() []ottl.Factory[*ottllog.TransformContext] {
	return slices.Collect(maps.Values(defaultLogFunctionsMap()))
}

// Deprecated: [v0.142.0] use DefaultMetricFunctionsNew.
func DefaultMetricFunctions() []ottl.Factory[ottlmetric.TransformContext] {
	return slices.Collect(maps.Values(ottlfuncs.StandardFuncs[ottlmetric.TransformContext]()))
}

func DefaultMetricFunctionsNew() []ottl.Factory[*ottlmetric.TransformContext] {
	return slices.Collect(maps.Values(defaultMetricFunctionsMap()))
}

// Deprecated: [v0.142.0] use DefaultDataPointFunctionsNew.
func DefaultDataPointFunctions() []ottl.Factory[ottldatapoint.TransformContext] {
	return slices.Collect(maps.Values(ottlfuncs.StandardFuncs[ottldatapoint.TransformContext]()))
}

func DefaultDataPointFunctionsNew() []ottl.Factory[*ottldatapoint.TransformContext] {
	return slices.Collect(maps.Values(defaultDataPointFunctionsMap()))
}

// Deprecated: [v0.142.0] use DefaultSpanFunctionsNew.
func DefaultSpanFunctions() []ottl.Factory[ottlspan.TransformContext] {
	functions := ottlfuncs.StandardFuncs[ottlspan.TransformContext]()

	spanFunctions := ottl.CreateFactoryMap(
		ottlfuncs.NewIsRootSpanFactory(),
		traces.NewSetSemconvSpanNameFactoryLegacy(),
	)

	maps.Copy(functions, spanFunctions)

	return slices.Collect(maps.Values(functions))
}

func DefaultSpanFunctionsNew() []ottl.Factory[*ottlspan.TransformContext] {
	return slices.Collect(maps.Values(defaultSpanFunctionsMap()))
}

// Deprecated: [v0.142.0] use DefaultSpanEventFunctionsNew.
func DefaultSpanEventFunctions() []ottl.Factory[ottlspanevent.TransformContext] {
	return slices.Collect(maps.Values(ottlfuncs.StandardFuncs[ottlspanevent.TransformContext]()))
}

func DefaultSpanEventFunctionsNew() []ottl.Factory[*ottlspanevent.TransformContext] {
	return slices.Collect(maps.Values(defaultSpanEventFunctionsMap()))
}

// Deprecated: [v0.145.0] use DefaultProfileFunctionsNew.
func DefaultProfileFunctions() []ottl.Factory[ottlprofile.TransformContext] {
	return slices.Collect(maps.Values(ottlfuncs.StandardFuncs[ottlprofile.TransformContext]()))
}

func DefaultProfileFunctionsNew() []ottl.Factory[*ottlprofile.TransformContext] {
	return slices.Collect(maps.Values(defaultProfileFunctionsMap()))
}

func defaultLogFunctionsMap() map[string]ottl.Factory[*ottllog.TransformContext] {
	return logs.LogFunctions()
}

func defaultMetricFunctionsMap() map[string]ottl.Factory[*ottlmetric.TransformContext] {
	return metrics.MetricFunctions()
}

func defaultDataPointFunctionsMap() map[string]ottl.Factory[*ottldatapoint.TransformContext] {
	return metrics.DataPointFunctions()
}

func defaultSpanFunctionsMap() map[string]ottl.Factory[*ottlspan.TransformContext] {
	return traces.SpanFunctions()
}

func defaultSpanEventFunctionsMap() map[string]ottl.Factory[*ottlspanevent.TransformContext] {
	return traces.SpanEventFunctions()
}

func defaultProfileFunctionsMap() map[string]ottl.Factory[*ottlprofile.TransformContext] {
	return profiles.ProfileFunctions()
}

func mergeFunctionsToMap[K any](functionMap map[string]ottl.Factory[K], functions []ottl.Factory[K]) map[string]ottl.Factory[K] {
	for _, f := range functions {
		functionMap[f.Name()] = f
	}
	return functionMap
}
