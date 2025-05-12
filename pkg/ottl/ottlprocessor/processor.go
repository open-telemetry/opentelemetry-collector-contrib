// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottlprocessor"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottldatapoint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspanevent"
)

type OttlProcessorFactory struct {
	AdditionalLogFunctions       []ottl.Factory[ottllog.TransformContext]
	AdditionalSpanFunctions      []ottl.Factory[ottlspan.TransformContext]
	AdditionalSpanEventFunctions []ottl.Factory[ottlspanevent.TransformContext]
	AdditionalMetricFunctions    []ottl.Factory[ottlmetric.TransformContext]
	AdditionalDataPointFunctions []ottl.Factory[ottldatapoint.TransformContext]
}

// FactoryOption applies changes to OttlProcessorFactory.
type OttlProcessorFactoryOption func(factory *OttlProcessorFactory)

// WithAdditionalMetricFunctions adds ottl metric functions to resulting processor
func WithAdditionalMetricFunctions(metricFunctions []ottl.Factory[ottlmetric.TransformContext]) OttlProcessorFactoryOption {
	return func(factory *OttlProcessorFactory) {
		factory.AdditionalMetricFunctions = metricFunctions
	}
}

// WithAdditionalLogFunctions adds ottl log functions to resulting processor
func WithAdditionalLogFunctions(logFunctions []ottl.Factory[ottllog.TransformContext]) OttlProcessorFactoryOption {
	return func(factory *OttlProcessorFactory) {
		factory.AdditionalLogFunctions = logFunctions
	}
}

// WithAdditionalSpanFunctions adds ottl span functions to resulting processor
func WithAdditionalSpanFunctions(spanFunctions []ottl.Factory[ottlspan.TransformContext]) OttlProcessorFactoryOption {
	return func(factory *OttlProcessorFactory) {
		factory.AdditionalSpanFunctions = spanFunctions
	}
}

// WithAdditionalSpanEventFunctions adds ottl span event functions to resulting processor
func WithAdditionalSpanEventFunctions(spanEventFunctions []ottl.Factory[ottlspanevent.TransformContext]) OttlProcessorFactoryOption {
	return func(factory *OttlProcessorFactory) {
		factory.AdditionalSpanEventFunctions = spanEventFunctions
	}
}

// WithAdditionalDataPointFunctions adds ottl data point functions to resulting processor
func WithAdditionalDataPointFunctions(dataPointFunctions []ottl.Factory[ottldatapoint.TransformContext]) OttlProcessorFactoryOption {
	return func(factory *OttlProcessorFactory) {
		factory.AdditionalDataPointFunctions = dataPointFunctions
	}
}
