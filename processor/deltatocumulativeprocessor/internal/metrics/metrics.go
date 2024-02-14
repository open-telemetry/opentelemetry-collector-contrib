// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/metrics"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil"
)

type Metric struct {
	res   pcommon.Resource
	scope pcommon.InstrumentationScope
	pmetric.Metric
}

func (m *Metric) Ident() Ident {
	id := Ident{
		ScopeIdent: ScopeIdent{
			ResourceIdent: ResourceIdent{
				attrs: pdatautil.MapHash(m.res.Attributes()),
			},
			name:    m.scope.Name(),
			version: m.scope.Version(),
			attrs:   pdatautil.MapHash(m.scope.Attributes()),
		},
		name: m.Metric.Name(),
		unit: m.Metric.Unit(),
		ty:   m.Metric.Type().String(),
	}

	switch m.Type() {
	case pmetric.MetricTypeSum:
		sum := m.Sum()
		id.monotonic = sum.IsMonotonic()
		id.temporality = sum.AggregationTemporality()
	case pmetric.MetricTypeExponentialHistogram:
		exp := m.ExponentialHistogram()
		id.monotonic = true
		id.temporality = exp.AggregationTemporality()
	case pmetric.MetricTypeHistogram:
		hist := m.Histogram()
		id.monotonic = true
		id.temporality = hist.AggregationTemporality()
	}

	return id
}

func From(res pcommon.Resource, scope pcommon.InstrumentationScope, metric pmetric.Metric) Metric {
	return Metric{res: res, scope: scope, Metric: metric}
}
