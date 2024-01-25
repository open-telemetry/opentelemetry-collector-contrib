package hostmetrics

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func addLoadMetrics(metrics pmetric.MetricSlice, dataset string) error {
	var timestamp pcommon.Timestamp
	var l1, l5, l15 float64

	for i := 0; i < metrics.Len(); i++ {
		metric := metrics.At(i)
		if metric.Name() == "system.cpu.load_average.1m" {
			dp := metric.Gauge().DataPoints().At(0)
			timestamp = dp.Timestamp()
			l1 = dp.DoubleValue()
		} else if metric.Name() == "system.cpu.load_average.5m" {
			dp := metric.Gauge().DataPoints().At(0)
			l5 = dp.DoubleValue()
		} else if metric.Name() == "system.cpu.load_average.15m" {
			dp := metric.Gauge().DataPoints().At(0)
			l15 = dp.DoubleValue()
		}
	}

	addMetrics(metrics, dataset,
		metric{
			dataType:    Gauge,
			name:        "system.load.1",
			timestamp:   timestamp,
			doubleValue: &l1,
		},
		metric{
			dataType:    Gauge,
			name:        "system.load.5",
			timestamp:   timestamp,
			doubleValue: &l5,
		},
		metric{
			dataType:    Gauge,
			name:        "system.load.15",
			timestamp:   timestamp,
			doubleValue: &l15,
		},
	)

	return nil
}
