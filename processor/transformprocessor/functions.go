// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package transformprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottldatapoint"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottllog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspanevent"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/logs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/metrics"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/traces"
	"go.uber.org/zap"
)

func DefaultLogFunctions() map[string]ottl.Factory[ottllog.TransformContext] {
	return logs.LogFunctions()
}

func DefaultMetricFunctions() map[string]ottl.Factory[ottlmetric.TransformContext] {
	return metrics.MetricFunctions()
}

func DefaultDataPointFunctions() map[string]ottl.Factory[ottldatapoint.TransformContext] {
	return metrics.DataPointFunctions()
}

func DefaultSpanFunctions() map[string]ottl.Factory[ottlspan.TransformContext] {
	return traces.SpanFunctions()
}

func DefaultSpanEventFunctions() map[string]ottl.Factory[ottlspanevent.TransformContext] {
	return traces.SpanEventFunctions()
}

func mergeAdditionalFunctions[T any](defaultFunctions map[string]ottl.Factory[T], additionalFunctions []ottl.Factory[T], logger *zap.Logger) map[string]ottl.Factory[T] {
	mergeResult := make(map[string]ottl.Factory[T], len(defaultFunctions))

	for name, fn := range defaultFunctions {
		mergeResult[name] = fn
	}

	for _, fn := range additionalFunctions {
		if _, ok := defaultFunctions[fn.Name()]; ok {
			logger.Sugar().Warnf("default function %s is overridden by custom function", fn.Name())
		} else {
			logger.Sugar().Infof("custom function %s is added", fn.Name())
		}
		mergeResult[fn.Name()] = fn
	}

	return mergeResult
}
