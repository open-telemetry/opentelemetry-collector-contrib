// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package transformprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor"

import (
	"maps"
	"slices"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottldatapoint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlexemplar"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspanevent"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/xprofile/ottlprofile"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/logs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/metrics"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/profiles"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/traces"
)

// DefaultLogFunctions returns the default OTTL functions available in the log context.
func DefaultLogFunctions() []ottl.Factory[*ottllog.TransformContext] {
	return slices.Collect(maps.Values(defaultLogFunctionsMap()))
}

// DefaultMetricFunctions returns the default OTTL functions available in the metric context.
func DefaultMetricFunctions() []ottl.Factory[*ottlmetric.TransformContext] {
	return slices.Collect(maps.Values(defaultMetricFunctionsMap()))
}

// DefaultDataPointFunctions returns the default OTTL functions available in the datapoint context.
func DefaultDataPointFunctions() []ottl.Factory[*ottldatapoint.TransformContext] {
	return slices.Collect(maps.Values(defaultDataPointFunctionsMap()))
}

// DefaultExemplarFunctions returns the default OTTL functions available in the exemplar context.
func DefaultExemplarFunctions() []ottl.Factory[*ottlexemplar.TransformContext] {
	return slices.Collect(maps.Values(defaultExemplarFunctionsMap()))
}

// DefaultSpanFunctions returns the default OTTL functions available in the span context.
func DefaultSpanFunctions() []ottl.Factory[*ottlspan.TransformContext] {
	return slices.Collect(maps.Values(defaultSpanFunctionsMap()))
}

// DefaultSpanEventFunctions returns the default OTTL functions available in the spanevent context.
func DefaultSpanEventFunctions() []ottl.Factory[*ottlspanevent.TransformContext] {
	return slices.Collect(maps.Values(defaultSpanEventFunctionsMap()))
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

func defaultExemplarFunctionsMap() map[string]ottl.Factory[*ottlexemplar.TransformContext] {
	return metrics.ExemplarFunctions()
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
