// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/metrics"

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
)

type convertSummaryQuantileValToGaugeArguments struct {
	AttributeKey ottl.Optional[string]
	Suffix       ottl.Optional[string]
}

func newConvertSummaryQuantileValToGaugeFactory() ottl.Factory[ottlmetric.TransformContext] {
	return ottl.NewFactory("convert_summary_quantile_val_to_gauge", &convertSummaryQuantileValToGaugeArguments{}, createConvertSummaryQuantileValToGaugeFunction)
}

func createConvertSummaryQuantileValToGaugeFunction(_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[ottlmetric.TransformContext], error) {
	args, ok := oArgs.(*convertSummaryQuantileValToGaugeArguments)

	if !ok {
		return nil, errors.New("convertSummaryQuantileValToGaugeFactory args must be of type *convertSummaryQuantileValToGaugeArguments")
	}

	return convertSummaryQuantileValToGauge(args.AttributeKey, args.Suffix)
}

func convertSummaryQuantileValToGauge(attrKey, suffix ottl.Optional[string]) (ottl.ExprFunc[ottlmetric.TransformContext], error) {
	metricNameSuffix := suffix.GetOr(".quantiles")
	attributeKey := attrKey.GetOr("quantile")

	return func(_ context.Context, tCtx ottlmetric.TransformContext) (any, error) {
		metric := tCtx.GetMetric()
		if metric.Type() != pmetric.MetricTypeSummary {
			return nil, nil
		}

		gaugeMetric := tCtx.GetMetrics().AppendEmpty()
		gaugeMetric.SetDescription(metric.Description())
		gaugeMetric.SetName(metric.Name() + metricNameSuffix)
		gaugeMetric.SetUnit(metric.Unit())
		gauge := gaugeMetric.SetEmptyGauge()

		dps := metric.Summary().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			dp := dps.At(i)
			for j := 0; j < dp.QuantileValues().Len(); j++ {
				q := dp.QuantileValues().At(j)
				gaugeDp := gauge.DataPoints().AppendEmpty()
				dp.Attributes().CopyTo(gaugeDp.Attributes())
				gaugeDp.Attributes().PutDouble(attributeKey, q.Quantile())
				gaugeDp.SetDoubleValue(q.Value())
				gaugeDp.SetStartTimestamp(dp.StartTimestamp())
				gaugeDp.SetTimestamp(dp.Timestamp())
			}
		}
		return nil, nil
	}, nil
}
