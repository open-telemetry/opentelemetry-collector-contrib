// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/metrics"

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/aggregateutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
)

type splitOnAttributeArguments struct {
	ByAttribute string
}

func newSplitOnAttributeFactory() ottl.Factory[ottlmetric.TransformContext] {
	return ottl.NewFactory("split_on_attribute", &splitOnAttributeArguments{}, createSplitOnAttributeFunction)
}

func createSplitOnAttributeFunction(_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[ottlmetric.TransformContext], error) {
	args, ok := oArgs.(*splitOnAttributeArguments)

	if !ok {
		return nil, errors.New("SplitOnAttributeFactory args must be of type *splitOnAttributeArguments")
	}

	return SplitOnAttribute(args.ByAttribute)
}

func SplitOnAttribute(byAttribute string) (ottl.ExprFunc[ottlmetric.TransformContext], error) {
	return func(_ context.Context, tCtx ottlmetric.TransformContext) (any, error) {
		cur := tCtx.GetMetric()
		metrics := tCtx.GetMetrics()
		newMetrics := map[string]pmetric.Metric{}

		// for each datapoint in `cur`, if `byAttribute` exists, copy the datapoint to a new metric
		// with the attribute value appended to it's name. remove `byAttribute` from each new datapoint.
		rangeDataPoints(cur, func(dp dataPointWrapper) bool {
			if attrValue, ok := dp.Attributes().Get(byAttribute); ok {
				newMetric, exists := newMetrics[attrValue.Str()]

				if !exists {
					newMetric = pmetric.NewMetric()
					aggregateutil.CopyMetricDetails(cur, newMetric)
					newMetric.SetName(fmt.Sprintf("%s.%s", cur.Name(), strings.ReplaceAll(attrValue.Str(), " ", "_")))
					newMetrics[attrValue.Str()] = newMetric
				}

				switch cur.Type() {
				case pmetric.MetricTypeGauge:
					newDp := newMetric.Gauge().DataPoints().AppendEmpty()
					dp.numberDataPointValue.CopyTo(newDp)
					newDp.Attributes().Remove(byAttribute)
				case pmetric.MetricTypeSum:
					newDp := newMetric.Sum().DataPoints().AppendEmpty()
					dp.numberDataPointValue.CopyTo(newDp)
					newDp.Attributes().Remove(byAttribute)
				case pmetric.MetricTypeHistogram:
					newDp := newMetric.Histogram().DataPoints().AppendEmpty()
					dp.histogramDataPointValue.CopyTo(newDp)
					newDp.Attributes().Remove(byAttribute)
				case pmetric.MetricTypeExponentialHistogram:
					newDp := newMetric.ExponentialHistogram().DataPoints().AppendEmpty()
					dp.expHistogramDataPointValue.CopyTo(newDp)
					newDp.Attributes().Remove(byAttribute)
				case pmetric.MetricTypeSummary:
					newDp := newMetric.Summary().DataPoints().AppendEmpty()
					dp.summaryDataPointValue.CopyTo(newDp)
					newDp.Attributes().Remove(byAttribute)
				}
			}
			return true
		})

		// we have to be careful about how we remove the split metric in the input transform context;
		// if we remove it, the outer loop misses the next metric because its position in the slice changes.
		// to combat this, we replace the current metric with the first new metric, and append the rest to
		// the end of the slice.
		replace := true
		for _, newMetric := range newMetrics {
			if replace {
				newMetric.CopyTo(cur)
				replace = false
			} else {
				newMetric.CopyTo(metrics.AppendEmpty())
			}
		}

		return nil, nil
	}, nil
}

type dataPointWrapper struct {
	metricType                 pmetric.MetricType
	numberDataPointValue       pmetric.NumberDataPoint
	histogramDataPointValue    pmetric.HistogramDataPoint
	expHistogramDataPointValue pmetric.ExponentialHistogramDataPoint
	summaryDataPointValue      pmetric.SummaryDataPoint
}

func (d dataPointWrapper) Attributes() pcommon.Map {
	switch d.metricType {
	case pmetric.MetricTypeGauge, pmetric.MetricTypeSum:
		return d.numberDataPointValue.Attributes()
	case pmetric.MetricTypeHistogram:
		return d.histogramDataPointValue.Attributes()
	case pmetric.MetricTypeExponentialHistogram:
		return d.expHistogramDataPointValue.Attributes()
	case pmetric.MetricTypeSummary:
		return d.summaryDataPointValue.Attributes()
	default:
		return pcommon.NewMap()
	}
}

func rangeDataPoints(metric pmetric.Metric, f func(dataPointWrapper) bool) {
	//exhaustive:enforce
	switch metric.Type() {
	case pmetric.MetricTypeGauge:
		for i := 0; i < metric.Gauge().DataPoints().Len(); i++ {
			dp := metric.Gauge().DataPoints().At(i)
			if !f(dataPointWrapper{metricType: pmetric.MetricTypeGauge, numberDataPointValue: dp}) {
				return
			}
		}
	case pmetric.MetricTypeSum:
		for i := 0; i < metric.Sum().DataPoints().Len(); i++ {
			dp := metric.Sum().DataPoints().At(i)
			if !f(dataPointWrapper{metricType: pmetric.MetricTypeSum, numberDataPointValue: dp}) {
				return
			}
		}
	case pmetric.MetricTypeHistogram:
		for i := 0; i < metric.Histogram().DataPoints().Len(); i++ {
			dp := metric.Histogram().DataPoints().At(i)
			if !f(dataPointWrapper{metricType: pmetric.MetricTypeHistogram, histogramDataPointValue: dp}) {
				return
			}
		}
	case pmetric.MetricTypeExponentialHistogram:
		for i := 0; i < metric.ExponentialHistogram().DataPoints().Len(); i++ {
			dp := metric.ExponentialHistogram().DataPoints().At(i)
			if !f(dataPointWrapper{metricType: pmetric.MetricTypeExponentialHistogram, expHistogramDataPointValue: dp}) {
				return
			}
		}
	case pmetric.MetricTypeSummary:
		for i := 0; i < metric.Summary().DataPoints().Len(); i++ {
			dp := metric.Summary().DataPoints().At(i)
			if !f(dataPointWrapper{metricType: pmetric.MetricTypeSummary, summaryDataPointValue: dp}) {
				return
			}
		}
	}
}
