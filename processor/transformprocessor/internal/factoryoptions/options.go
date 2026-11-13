// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package factoryoptions // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/factoryoptions"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottldatapoint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlexemplar"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspanevent"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/xprofile/ottlprofile"
)

// Options holds the OTTL functions registered with the transform processor factory.
type Options struct {
	DataPointFunctions                  map[string]ottl.Factory[*ottldatapoint.TransformContext]
	ExemplarFunctions                   map[string]ottl.Factory[*ottlexemplar.TransformContext]
	LogFunctions                        map[string]ottl.Factory[*ottllog.TransformContext]
	MetricFunctions                     map[string]ottl.Factory[*ottlmetric.TransformContext]
	SpanEventFunctions                  map[string]ottl.Factory[*ottlspanevent.TransformContext]
	SpanFunctions                       map[string]ottl.Factory[*ottlspan.TransformContext]
	ProfileFunctions                    map[string]ottl.Factory[*ottlprofile.TransformContext]
	DefaultDataPointFunctionsOverridden bool
	DefaultExemplarFunctionsOverridden  bool
	DefaultLogFunctionsOverridden       bool
	DefaultMetricFunctionsOverridden    bool
	DefaultSpanEventFunctionsOverridden bool
	DefaultSpanFunctionsOverridden      bool
	DefaultProfileFunctionsOverridden   bool
}

func MergeFunctionsToMap[K any](functionMap map[string]ottl.Factory[K], functions []ottl.Factory[K]) map[string]ottl.Factory[K] {
	for _, f := range functions {
		functionMap[f.Name()] = f
	}
	return functionMap
}
