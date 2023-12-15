// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/metrics"

import (
	"context"

	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
)

func newConvertSumToGaugeFactory() ottl.Factory[ottlmetric.TransformContext] {
	return ottl.NewFactory("convert_sum_to_gauge", nil, createConvertSumToGaugeFunction)
}

func createConvertSumToGaugeFunction(_ ottl.FunctionContext, _ ottl.Arguments) (ottl.ExprFunc[ottlmetric.TransformContext], error) {
	return convertSumToGauge()
}

func convertSumToGauge() (ottl.ExprFunc[ottlmetric.TransformContext], error) {
	return func(_ context.Context, tCtx ottlmetric.TransformContext) (any, error) {
		metric := tCtx.GetMetric()
		if metric.Type() != pmetric.MetricTypeSum {
			return nil, nil
		}

		dps := metric.Sum().DataPoints()

		// Setting the data type removed all the data points, so we must copy them back to the metric.
		dps.CopyTo(metric.SetEmptyGauge().DataPoints())

		return nil, nil
	}, nil
}
