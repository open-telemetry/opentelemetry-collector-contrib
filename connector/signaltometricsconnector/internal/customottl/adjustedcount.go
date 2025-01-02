// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package customottl // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/signaltometricsconnector/internal/customottl"

import (
	"context"
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlspan"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling"
)

func NewAdjustedCountFactory() ottl.Factory[ottlspan.TransformContext] {
	return ottl.NewFactory("AdjustedCount", nil, createAdjustedCountFunction)
}

func createAdjustedCountFunction(_ ottl.FunctionContext, _ ottl.Arguments) (ottl.ExprFunc[ottlspan.TransformContext], error) {
	return adjustedCount()
}

func adjustedCount() (ottl.ExprFunc[ottlspan.TransformContext], error) {
	return func(_ context.Context, tCtx ottlspan.TransformContext) (any, error) {
		tracestate := tCtx.GetSpan().TraceState().AsRaw()
		w3cTraceState, err := sampling.NewW3CTraceState(tracestate)
		if err != nil {
			return float64(0), fmt.Errorf("failed to parse w3c tracestate: %w", err)
		}
		otTraceState := w3cTraceState.OTelValue()
		if otTraceState == nil {
			// If otel trace state is missing, default to 1
			return float64(1), nil
		}
		if len(otTraceState.TValue()) == 0 {
			// For non-probabilistic sampler OR always sampling threshold, default to 1
			return float64(1), nil
		}
		return otTraceState.AdjustedCount(), nil
	}, nil
}
