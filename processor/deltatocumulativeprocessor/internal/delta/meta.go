package delta

import (
	"sync"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor/internal/identity"
)

var _ Aggregator = (*Metadata)(nil)

const metaSeriesKey = "internal.deltatocumulativeprocessor.series"

type Metadata struct {
	mu sync.Mutex

	next  Aggregator
	attrs map[identity.Metric]map[identity.Series]pcommon.Map
}

func (m *Metadata) Aggregate(id identity.Metric, dps pmetric.NumberDataPointSlice) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	series, ok := m.attrs[id]
	if !ok {
		series = make(map[identity.Series]pcommon.Map)
		m.attrs[id] = series
	}

	for i := 0; i < dps.Len(); i++ {
		dp := dps.At(i)
		id := identity.OfSeries(dp.Attributes())
		if _, ok := series[id]; !ok {
			series[id] = dp.Attributes()
		}
	}

	return m.next.Aggregate(id, dps)
}

func (m *Metadata) Value(id identity.Metric) pmetric.NumberDataPointSlice {
	m.mu.Lock()
	defer m.mu.Unlock()

	dps := m.next.Value(id)
	for i := 0; i < dps.Len(); i++ {
		dp := dps.At(i)
		series, ok := dp.Attributes().AsRaw()[metaSeriesKey].(identity.Series)
		if !ok {
			continue
		}
		attrs, ok := m.attrs[id][series]
		if !ok {
			continue
		}
		delete(dp.Attributes().AsRaw(), metaSeriesKey)
		attrs.CopyTo(dp.Attributes())
	}
	return dps
}
