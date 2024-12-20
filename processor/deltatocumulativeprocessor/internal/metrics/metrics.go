// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/metrics"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/identity"
)

type Ident = identity.Metric

type Metric struct {
	res   pcommon.Resource
	scope pcommon.InstrumentationScope
	pmetric.Metric
}

func (m *Metric) Ident() Ident {
	return identity.OfResourceMetric(m.res, m.scope, m.Metric)
}

func (m *Metric) Resource() pcommon.Resource {
	return m.res
}

func (m *Metric) Scope() pcommon.InstrumentationScope {
	return m.scope
}

func From(res pcommon.Resource, scope pcommon.InstrumentationScope, metric pmetric.Metric) Metric {
	return Metric{res: res, scope: scope, Metric: metric}
}

func (m Metric) AggregationTemporality() pmetric.AggregationTemporality {
	switch m.Type() {
	case pmetric.MetricTypeSum:
		return m.Sum().AggregationTemporality()
	case pmetric.MetricTypeHistogram:
		return m.Histogram().AggregationTemporality()
	case pmetric.MetricTypeExponentialHistogram:
		return m.ExponentialHistogram().AggregationTemporality()
	}

	return pmetric.AggregationTemporalityUnspecified
}

func (m Metric) Typed() any {
	//exhaustive:enforce
	switch m.Type() {
	case pmetric.MetricTypeSum:
		return Sum(m)
	case pmetric.MetricTypeGauge:
		return Gauge(m)
	case pmetric.MetricTypeExponentialHistogram:
		return ExpHistogram(m)
	case pmetric.MetricTypeHistogram:
		return Histogram(m)
	case pmetric.MetricTypeSummary:
		return Summary(m)
	}
	panic("unreachable")
}
