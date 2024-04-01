// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/identity"
)

type Metric struct {
	res            pcommon.Resource
	resSchemaURL   string
	scope          pcommon.InstrumentationScope
	scopeSchemaURL string
	innerMetric    pmetric.Metric
}

func From(res pcommon.Resource, resSchemaURL string, scope pcommon.InstrumentationScope, scopeSchemaURL string, metric pmetric.Metric) Metric {
	newMetric := Metric{
		res:            res,
		resSchemaURL:   resSchemaURL,
		scope:          scope,
		scopeSchemaURL: scopeSchemaURL,
		innerMetric:    pmetric.NewMetric(),
	}

	// Clone the metric metadata without the datapoints
	newMetric.innerMetric.SetName(metric.Name())
	newMetric.innerMetric.SetDescription(metric.Description())
	newMetric.innerMetric.SetUnit(metric.Unit())
	switch metric.Type() {
	case pmetric.MetricTypeGauge:
		newMetric.innerMetric.SetEmptyGauge()
	case pmetric.MetricTypeSum:
		sum := metric.Sum()

		newSum := newMetric.innerMetric.SetEmptySum()
		newSum.SetAggregationTemporality(sum.AggregationTemporality())
		newSum.SetIsMonotonic(sum.IsMonotonic())
	case pmetric.MetricTypeHistogram:
		histogram := metric.Histogram()

		newHistogram := newMetric.innerMetric.SetEmptyHistogram()
		newHistogram.SetAggregationTemporality(histogram.AggregationTemporality())
	case pmetric.MetricTypeExponentialHistogram:
		expHistogram := metric.ExponentialHistogram()

		newExpHistogram := newMetric.innerMetric.SetEmptyExponentialHistogram()
		newExpHistogram.SetAggregationTemporality(expHistogram.AggregationTemporality())
	case pmetric.MetricTypeSummary:
		newMetric.innerMetric.SetEmptySummary()
	}

	return newMetric
}

func (m *Metric) Identity() identity.Metric {
	return identity.OfResourceMetric(m.res, m.scope, m.innerMetric)
}

func (m *Metric) CopyToResourceMetric(dest pmetric.ResourceMetrics) {
	m.res.CopyTo(dest.Resource())
	dest.SetSchemaUrl(m.resSchemaURL)
}

func (m *Metric) CopyToScopeMetric(dest pmetric.ScopeMetrics) {
	m.scope.CopyTo(dest.Scope())
	dest.SetSchemaUrl(m.scopeSchemaURL)
}

func (m *Metric) CopyToPMetric(dest pmetric.Metric) {
	m.innerMetric.CopyTo(dest)
}

type DataPointSlice[DP DataPoint[DP]] interface {
	Len() int
	At(i int) DP
}

type DataPoint[Self any] interface {
	pmetric.NumberDataPoint | pmetric.HistogramDataPoint | pmetric.ExponentialHistogramDataPoint

	Timestamp() pcommon.Timestamp
	Attributes() pcommon.Map
	CopyTo(dest Self)
}

type StreamDataPoint[DP DataPoint[DP]] struct {
	Metric      Metric
	DataPoint   DP
	LastUpdated time.Time
}

func StreamDataPointIdentity[DP DataPoint[DP]](metricID identity.Metric, dataPoint DP) identity.Stream {
	return identity.OfStream[DP](metricID, dataPoint)
}
