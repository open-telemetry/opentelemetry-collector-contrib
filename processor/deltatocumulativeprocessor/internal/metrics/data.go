// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/metrics"

import (
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data"
)

type Data[D data.Point[D]] interface {
	At(i int) D
	Len() int
	Ident() Ident
}

type Filterable[D data.Point[D]] interface {
	Data[D]
	Filter(func(D) bool)
}

type Sum Metric

func (s Sum) At(i int) data.Number {
	dp := Metric(s).Sum().DataPoints().At(i)
	return data.Number{NumberDataPoint: dp}
}

func (s Sum) Len() int {
	return Metric(s).Sum().DataPoints().Len()
}

func (s Sum) Ident() Ident {
	return (*Metric)(&s).Ident()
}

func (s Sum) Filter(expr func(data.Number) bool) {
	s.Sum().DataPoints().RemoveIf(func(dp pmetric.NumberDataPoint) bool {
		return !expr(data.Number{NumberDataPoint: dp})
	})
}

func (s Sum) SetAggregationTemporality(at pmetric.AggregationTemporality) {
	s.Sum().SetAggregationTemporality(at)
}

type Histogram Metric

func (s Histogram) At(i int) data.Histogram {
	dp := Metric(s).Histogram().DataPoints().At(i)
	return data.Histogram{HistogramDataPoint: dp}
}

func (s Histogram) Len() int {
	return Metric(s).Histogram().DataPoints().Len()
}

func (s Histogram) Ident() Ident {
	return (*Metric)(&s).Ident()
}

func (s Histogram) Filter(expr func(data.Histogram) bool) {
	s.Histogram().DataPoints().RemoveIf(func(dp pmetric.HistogramDataPoint) bool {
		return !expr(data.Histogram{HistogramDataPoint: dp})
	})
}

func (s Histogram) SetAggregationTemporality(at pmetric.AggregationTemporality) {
	s.Histogram().SetAggregationTemporality(at)
}

type ExpHistogram Metric

func (s ExpHistogram) At(i int) data.ExpHistogram {
	dp := Metric(s).ExponentialHistogram().DataPoints().At(i)
	return data.ExpHistogram{DataPoint: dp}
}

func (s ExpHistogram) Len() int {
	return Metric(s).ExponentialHistogram().DataPoints().Len()
}

func (s ExpHistogram) Ident() Ident {
	return (*Metric)(&s).Ident()
}

func (s ExpHistogram) Filter(expr func(data.ExpHistogram) bool) {
	s.ExponentialHistogram().DataPoints().RemoveIf(func(dp pmetric.ExponentialHistogramDataPoint) bool {
		return !expr(data.ExpHistogram{DataPoint: dp})
	})
}

func (s ExpHistogram) SetAggregationTemporality(at pmetric.AggregationTemporality) {
	s.ExponentialHistogram().SetAggregationTemporality(at)
}

type Gauge Metric

func (s Gauge) At(i int) data.Number {
	dp := Metric(s).Gauge().DataPoints().At(i)
	return data.Number{NumberDataPoint: dp}
}

func (s Gauge) Len() int {
	return Metric(s).Gauge().DataPoints().Len()
}

func (s Gauge) Ident() Ident {
	return (*Metric)(&s).Ident()
}

func (s Gauge) Filter(expr func(data.Number) bool) {
	s.Gauge().DataPoints().RemoveIf(func(dp pmetric.NumberDataPoint) bool {
		return !expr(data.Number{NumberDataPoint: dp})
	})
}

type Summary Metric

func (s Summary) At(i int) data.Summary {
	dp := Metric(s).Summary().DataPoints().At(i)
	return data.Summary{SummaryDataPoint: dp}
}

func (s Summary) Len() int {
	return Metric(s).Summary().DataPoints().Len()
}

func (s Summary) Ident() Ident {
	return (*Metric)(&s).Ident()
}

func (s Summary) Filter(expr func(data.Summary) bool) {
	s.Summary().DataPoints().RemoveIf(func(dp pmetric.SummaryDataPoint) bool {
		return !expr(data.Summary{SummaryDataPoint: dp})
	})
}
