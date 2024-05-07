// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package gmetric // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/reorderprocessor/internal/gmetric"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

type Metric[T any, S Slice[S, T]] interface {
	Gauge | Sum | Expo | Hist | Summ
	DataPoints() S
}

type Slice[Self any, E any] interface {
	pmetric.NumberDataPointSlice | pmetric.ExponentialHistogramDataPointSlice | pmetric.HistogramDataPointSlice | pmetric.SummaryDataPointSlice
	List[E]
	AppendEmpty() E
	MoveAndAppendTo(Self)
}

type (
	Gauge pmetric.Metric
	Sum   pmetric.Metric
	Expo  pmetric.Metric
	Hist  pmetric.Metric
	Summ  pmetric.Metric
)

func (s Sum) DataPoints() pmetric.NumberDataPointSlice {
	return pmetric.Metric(s).Sum().DataPoints()
}

func (s Gauge) DataPoints() pmetric.NumberDataPointSlice {
	return pmetric.Metric(s).Gauge().DataPoints()
}

func (s Expo) DataPoints() pmetric.ExponentialHistogramDataPointSlice {
	return pmetric.Metric(s).ExponentialHistogram().DataPoints()
}

func (s Hist) DataPoints() pmetric.HistogramDataPointSlice {
	return pmetric.Metric(s).Histogram().DataPoints()
}

func (s Summ) DataPoints() pmetric.SummaryDataPointSlice {
	return pmetric.Metric(s).Summary().DataPoints()
}

func Empty[T any, S Slice[S, T]]() S {
	s := new(S)

	switch t := any(s).(type) {
	case *pmetric.NumberDataPointSlice:
		*t = pmetric.NewNumberDataPointSlice()
	case *pmetric.ExponentialHistogramDataPointSlice:
		*t = pmetric.NewExponentialHistogramDataPointSlice()
	case *pmetric.HistogramDataPointSlice:
		*t = pmetric.NewHistogramDataPointSlice()
	case *pmetric.SummaryDataPointSlice:
		*t = pmetric.NewSummaryDataPointSlice()
	}

	return *s
}

func Clear[T any, S Slice[S, T]](s S) {
	s.MoveAndAppendTo(Empty[T, S]())
}

type Point[Self any] interface {
	Attributes() pcommon.Map
	Timestamp() pcommon.Timestamp
	MoveTo(Self)
}

func Typed(m pmetric.Metric) any {
	//exhaustive:enforce
	switch m.Type() {
	case pmetric.MetricTypeGauge:
		return Gauge(m)
	case pmetric.MetricTypeSum:
		return Sum(m)
	case pmetric.MetricTypeHistogram:
		return Hist(m)
	case pmetric.MetricTypeExponentialHistogram:
		return Expo(m)
	case pmetric.MetricTypeSummary:
		return Summ(m)
	}
	panic("unreachable")
}
