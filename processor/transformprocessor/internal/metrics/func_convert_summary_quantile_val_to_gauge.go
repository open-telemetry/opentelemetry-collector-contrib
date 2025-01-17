// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/metrics"

import (
	"context"
	"fmt"
	"strconv"

	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottldatapoint"
)

type convertSummaryQuantileValToGaugeArguments struct {
	AttributeKey ottl.Optional[string]
}

func newConvertSummaryQuantileValToGaugeFactory() ottl.Factory[ottldatapoint.TransformContext] {
	return ottl.NewFactory("convert_summary_quantile_val_to_gauge", &convertSummaryQuantileValToGaugeArguments{}, createConvertSummaryQuantileValToGaugeFunction)
}

func createConvertSummaryQuantileValToGaugeFunction(_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[ottldatapoint.TransformContext], error) {
	args, ok := oArgs.(*convertSummaryQuantileValToGaugeArguments)

	if !ok {
		return nil, fmt.Errorf("convertSummaryQuantileValToGaugeFactory args must be of type *convertSummaryQuantileValToGaugeArguments")
	}

	return convertSummaryQuantileValToGauge(args.AttributeKey)
}

func convertSummaryQuantileValToGauge(attrKey ottl.Optional[string]) (ottl.ExprFunc[ottldatapoint.TransformContext], error) {
	attributeKey := "quantile"
	if !attrKey.IsEmpty() {
		attributeKey = attrKey.Get()
	}
	return func(_ context.Context, tCtx ottldatapoint.TransformContext) (any, error) {
		metric := tCtx.GetMetric()
		if metric.Type() != pmetric.MetricTypeSummary {
			return nil, nil
		}

		gaugeMetric := tCtx.GetMetrics().AppendEmpty()
		gaugeMetric.SetDescription(metric.Description())
		gaugeMetric.SetName(metric.Name())
		gaugeMetric.SetUnit(metric.Unit())
		gauge := gaugeMetric.SetEmptyGauge()

		dps := metric.Summary().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			dp := dps.At(i)
			for j := 0; j < dp.QuantileValues().Len(); j++ {
				q := dp.QuantileValues().At(j)
				gaugeDp := gauge.DataPoints().AppendEmpty()
				dp.Attributes().CopyTo(gaugeDp.Attributes())
				gaugeDp.Attributes().PutStr(attributeKey, quantileToStringValue(q.Quantile()))
				gaugeDp.SetDoubleValue(q.Value())
				gaugeDp.SetStartTimestamp(dp.StartTimestamp())
				gaugeDp.SetTimestamp(dp.Timestamp())
			}
		}
		return nil, nil
	}, nil
}

func quantileToStringValue(q float64) string {
	result := strconv.FormatFloat(q, 'f', -1, 64)
	if len(result) < 4 && result != "0" && result != "1" {
		result += "0"
	}
	return result
}
