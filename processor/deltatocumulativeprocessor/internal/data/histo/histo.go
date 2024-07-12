package histo

import "go.opentelemetry.io/collector/pdata/pmetric"

type DataPoint = pmetric.HistogramDataPoint

type Bounds []float64

func (b Bounds) Observe(dps ...float64) DataPoint {
	panic("todo")
}
