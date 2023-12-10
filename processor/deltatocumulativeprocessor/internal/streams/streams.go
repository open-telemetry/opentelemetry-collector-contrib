package streams

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/metrics"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

type Aggregator interface {
	Aggregate(Ident, pmetric.NumberDataPoint)
	Value(Ident) pmetric.NumberDataPoint
}

func Samples(m metrics.Metric, fn func(meta Meta, dp pmetric.NumberDataPoint)) {
	metricMeta := m.Meta()
	dps := m.Sum().DataPoints()
	for i := 0; i < dps.Len(); i++ {
		dp := dps.At(i)
		meta := Meta{metric: metricMeta, attrs: dp.Attributes()}
		fn(meta, dp)
	}
}

type Ident struct {
	metric metrics.Ident
	attrs  [16]byte
}

type Meta struct {
	metric metrics.Meta
	attrs  pcommon.Map
}

func (m Meta) Identity() Ident {
	return Ident{
		metric: m.metric.Identity(),
		attrs:  pdatautil.MapHash(m.attrs),
	}
}
