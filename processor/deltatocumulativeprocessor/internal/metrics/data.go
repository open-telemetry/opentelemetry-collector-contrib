// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/metrics"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/data"
)

type Data[D data.Point[D]] interface {
	At(i int) D
	Len() int
	Ident() Ident
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

type ExpHistogram Metric

func (s ExpHistogram) At(i int) data.ExpHistogram {
	dp := Metric(s).ExponentialHistogram().DataPoints().At(i)
	return data.ExpHistogram{ExponentialHistogramDataPoint: dp}
}

func (s ExpHistogram) Len() int {
	return Metric(s).ExponentialHistogram().DataPoints().Len()
}

func (s ExpHistogram) Ident() Ident {
	return (*Metric)(&s).Ident()
}
