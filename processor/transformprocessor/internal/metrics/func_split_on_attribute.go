// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/metrics"

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/aggregateutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlmetric"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
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

		rangeDataPoints(cur, func(dp dataPointWrapper) bool {
			if attrValue, ok := dp.Attributes().Get(byAttribute); ok {
				newMetric, exists := newMetrics[attrValue.Str()]

				if !exists {
					newMetric = metrics.AppendEmpty()
					aggregateutil.CopyMetricDetails(cur, newMetric)
					newMetric.SetName(fmt.Sprintf("%s.%s", cur.Name(), strings.ReplaceAll(attrValue.Str(), " ", "_")))
					newMetrics[attrValue.Str()] = newMetric
				}

				switch cur.Type() {
				case pmetric.MetricTypeGauge:
					if !exists {
						newMetric.SetEmptyGauge()
					}
					newDp := newMetric.Gauge().DataPoints().AppendEmpty()
					dp.numberDataPointValue.CopyTo(newDp)
					newDp.Attributes().Remove(byAttribute)
				case pmetric.MetricTypeSum:
					if !exists {
						newMetric.SetEmptySum()
					}
					newDp := newMetric.Sum().DataPoints().AppendEmpty()
					dp.numberDataPointValue.CopyTo(newDp)
					newDp.Attributes().Remove(byAttribute)
				case pmetric.MetricTypeHistogram:
					if !exists {
						newMetric.SetEmptyHistogram()
					}
					newDp := newMetric.Histogram().DataPoints().AppendEmpty()
					dp.histogramDataPointValue.CopyTo(newDp)
					newDp.Attributes().Remove(byAttribute)
				case pmetric.MetricTypeExponentialHistogram:
					if !exists {
						newMetric.SetEmptyExponentialHistogram()
					}
					newDp := newMetric.ExponentialHistogram().DataPoints().AppendEmpty()
					dp.expHistogramDataPointValue.CopyTo(newDp)
					newDp.Attributes().Remove(byAttribute)
				case pmetric.MetricTypeSummary:
					if !exists {
						newMetric.SetEmptySummary()
					}
					newDp := newMetric.Summary().DataPoints().AppendEmpty()
					dp.summaryDataPointValue.CopyTo(newDp)
					newDp.Attributes().Remove(byAttribute)
				}
			}
			return true
		})

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
