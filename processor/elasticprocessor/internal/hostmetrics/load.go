package hostmetrics

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func addLoadMetrics(metrics pmetric.MetricSlice) error {
	var timestamp pcommon.Timestamp
	var l1, l5, l15 float64
	cpus := map[string]bool{}

	for i := 0; i < metrics.Len(); i++ {
		metric := metrics.At(i)
		if metric.Name() == "system.cpu.load_average.1m" {
			dp := metric.Gauge().DataPoints().At(0)
			l1 = dp.DoubleValue()

			// common values
			timestamp = dp.Timestamp()
			if cpu, ok := dp.Attributes().Get("cpu"); ok {
				cpus[cpu.Str()] = true
			}
		} else if metric.Name() == "system.cpu.load_average.5m" {
			dp := metric.Gauge().DataPoints().At(0)
			l5 = dp.DoubleValue()
		} else if metric.Name() == "system.cpu.load_average.15m" {
			dp := metric.Gauge().DataPoints().At(0)
			l15 = dp.DoubleValue()
		}
	}

	cores := int64(len(cpus))
	l1norm := l1 / float64(cores)
	l5norm := l5 / float64(cores)
	l15norm := l15 / float64(cores)

	addMetrics(metrics,
		metric{
			dataType:  Sum,
			name:      "system.load.cores",
			timestamp: timestamp,
			intValue:  &cores,
		},
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
		metric{
			dataType:    Gauge,
			name:        "system.load.norm.1",
			timestamp:   timestamp,
			doubleValue: &l1norm,
		},
		metric{
			dataType:    Gauge,
			name:        "system.load.norm.5",
			timestamp:   timestamp,
			doubleValue: &l5norm,
		},
		metric{
			dataType:    Gauge,
			name:        "system.load.norm.15",
			timestamp:   timestamp,
			doubleValue: &l15norm,
		},
	)

	return nil
}
