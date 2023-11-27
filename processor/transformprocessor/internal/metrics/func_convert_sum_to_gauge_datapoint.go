// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/metrics"

import (
	"context"

	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottldatapoint"
)

func newConvertDatapointSumToGaugeFactory() ottl.Factory[ottldatapoint.TransformContext] {
	return ottl.NewFactory("convert_sum_to_gauge", nil, createDatapointConvertSumToGaugeFunction)
}

func createDatapointConvertSumToGaugeFunction(_ ottl.FunctionContext, _ ottl.Arguments) (ottl.ExprFunc[ottldatapoint.TransformContext], error) {
	return convertDatapointSumToGauge()
}

func convertDatapointSumToGauge() (ottl.ExprFunc[ottldatapoint.TransformContext], error) {
	return func(_ context.Context, tCtx ottldatapoint.TransformContext) (any, error) {
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
