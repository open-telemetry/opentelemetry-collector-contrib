package metrics

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

type Meta struct {
	resource pcommon.Resource
	scope    pcommon.InstrumentationScope
	metric   pmetric.Metric

	// intrinsic
	monotonic   bool
	temporality pmetric.AggregationTemporality
}

func (m Meta) Resource() pcommon.Resource {
	return m.resource
}

func (m Meta) Scope() pcommon.InstrumentationScope {
	return m.scope
}

func (m Meta) CopyTo(dst pmetric.Metric) {
	m.metric.CopyTo(dst)
	switch dst.Type() {
	case pmetric.MetricTypeSum:
		dst.Sum().SetIsMonotonic(m.monotonic)
		dst.Sum().SetAggregationTemporality(m.temporality)
	case pmetric.MetricTypeHistogram:
		dst.Histogram().SetAggregationTemporality(m.temporality)
	case pmetric.MetricTypeExponentialHistogram:
		dst.ExponentialHistogram().SetAggregationTemporality(m.temporality)
	}
}

func (m Meta) Identity() Ident {
	return Ident{
		ScopeIdent: ScopeIdent{
			ResourceIdent: ResourceIdent{
				attrs: pdatautil.MapHash(m.resource.Attributes()),
			},
			name:    m.scope.Name(),
			version: m.scope.Version(),
			attrs:   pdatautil.MapHash(m.scope.Attributes()),
		},
		name: m.metric.Name(),
		unit: m.metric.Unit(),
		ty:   m.metric.Type().String(),

		monotonic:   m.monotonic,
		temporality: m.temporality,
	}
}
