package hostmetrics

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func addMemoryMetrics(metrics pmetric.MetricSlice, dataset string) error {
	var timestamp pcommon.Timestamp
	var total, free, cached, usedBytes, actualFree, actualUsedBytes int64
	var usedPercent, actualUsedPercent float64

	// iterate all metrics in the current scope and generate the additional Elastic system integration metrics
	for i := 0; i < metrics.Len(); i++ {
		metric := metrics.At(i)
		if metric.Name() == "system.memory.usage" {
			dataPoints := metric.Sum().DataPoints()
			for j := 0; j < dataPoints.Len(); j++ {
				dp := dataPoints.At(j)

				timestamp = dp.Timestamp()

				value := dp.IntValue()
				if state, ok := dp.Attributes().Get("state"); ok {
					switch state.Str() {
					case "cached":
						cached = value
						total += value
					case "free":
						free = value
						usedBytes -= value
						total += value
					case "used":
						total += value
						actualUsedBytes += value
					case "buffered":
						total += value
						actualUsedBytes += value
					case "slab_unreclaimable":
						actualUsedBytes += value
					case "slab_reclaimable":
						actualUsedBytes += value
					}
				}
			}
		} else if metric.Name() == "system.memory.utilization" {
			dataPoints := metric.Gauge().DataPoints()
			for j := 0; j < dataPoints.Len(); j++ {
				dp := dataPoints.At(j)
				value := dp.DoubleValue()

				if state, ok := dp.Attributes().Get("state"); ok {
					switch state.Str() {
					case "free":
						usedPercent = 1 - value
					case "used":
						actualUsedPercent += value
					case "buffered":
						actualUsedPercent += value
					case "slab_unreclaimable":
						actualUsedPercent += value
					case "slab_reclaimable":
						actualUsedPercent += value
					}
				}
			}
		}
	}

	usedBytes += total
	actualFree = total - actualUsedBytes

	addMetrics(metrics, dataset,
		metric{
			dataType:  Sum,
			name:      "system.memory.total",
			timestamp: timestamp,
			intValue:  &total,
		},
		metric{
			dataType:  Sum,
			name:      "system.memory.free",
			timestamp: timestamp,
			intValue:  &free,
		},
		metric{
			dataType:  Sum,
			name:      "system.memory.cached",
			timestamp: timestamp,
			intValue:  &cached,
		},
		metric{
			dataType:  Sum,
			name:      "system.memory.used.bytes",
			timestamp: timestamp,
			intValue:  &usedBytes,
		},
		metric{
			dataType:  Sum,
			name:      "system.memory.actual.used.bytes",
			timestamp: timestamp,
			intValue:  &actualUsedBytes,
		},
		metric{
			dataType:  Sum,
			name:      "system.memory.actual.free",
			timestamp: timestamp,
			intValue:  &actualFree,
		},
		metric{
			dataType:    Gauge,
			name:        "system.memory.used.pct",
			timestamp:   timestamp,
			doubleValue: &usedPercent,
		},
		metric{
			dataType:    Gauge,
			name:        "system.memory.actual.used.pct",
			timestamp:   timestamp,
			doubleValue: &actualUsedPercent,
		},
	)

	return nil
}
