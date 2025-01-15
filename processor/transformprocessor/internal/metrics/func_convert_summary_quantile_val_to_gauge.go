// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/metrics"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottldatapoint"
)

type convertSummaryQuantileValToGaugeArguments struct {
	Suffix ottl.Optional[string]
}

func newConvertSummaryQuantileValToGaugeFactory() ottl.Factory[ottldatapoint.TransformContext] {
	return ottl.NewFactory("convert_summary_quantile_val_to_gauge", &convertSummaryQuantileValToGaugeArguments{}, createConvertSummaryQuantileValToGaugeFunction)
}

func createConvertSummaryQuantileValToGaugeFunction(_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[ottldatapoint.TransformContext], error) {
	args, ok := oArgs.(*convertSummaryQuantileValToGaugeArguments)

	if !ok {
		return nil, fmt.Errorf("convertSummaryQuantileValToGaugeFactory args must be of type *convertSummaryQuantileValToGaugeArguments")
	}

	return convertSummaryQuantileValToGauge(args.Suffix)
}

func convertSummaryQuantileValToGauge(suffix ottl.Optional[string]) (ottl.ExprFunc[ottldatapoint.TransformContext], error) {
	metricNameSuffix := ".quantile"
	if !suffix.IsEmpty() {
		metricNameSuffix = suffix.Get()
	}
	return func(_ context.Context, tCtx ottldatapoint.TransformContext) (any, error) {
		metric := tCtx.GetMetric()
		if metric.Type() != pmetric.MetricTypeSummary {
			return nil, nil
		}

		gaugeMetric := tCtx.GetMetrics().AppendEmpty()
		gaugeMetric.SetDescription(metric.Description())
		gaugeMetric.SetName(metric.Name() + metricNameSuffix)
		gaugeMetric.SetUnit(metric.Unit())

		gaugeDps := gaugeMetric.Gauge().DataPoints()
		dps := metric.Summary().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			dp := dps.At(i)
			gaugeDp := gaugeDps.AppendEmpty()
			dp.Attributes().CopyTo(gaugeDp.Attributes())
			gaugeDp.SetDoubleValue(2)
			gaugeDp.SetIntValue(1)
			gaugeDp.SetStartTimestamp(dp.StartTimestamp())
			gaugeDp.SetTimestamp(dp.Timestamp())
		}
		return nil, nil
	}, nil
}
