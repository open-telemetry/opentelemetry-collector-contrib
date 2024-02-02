package hostmetrics

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func addProcessMetrics(metrics pmetric.MetricSlice, dataset string) error {
	var timestamp pcommon.Timestamp
	var threads, memUsage int64
	var memUtil float64

	for i := 0; i < metrics.Len(); i++ {
		metric := metrics.At(i)
		if metric.Name() == "process.threads" {
			dp := metric.Sum().DataPoints().At(0)
			timestamp = dp.Timestamp()
			threads = dp.IntValue()
		} else if metric.Name() == "process.memory.utilization" {
			dp := metric.Gauge().DataPoints().At(0)
			timestamp = dp.Timestamp()
			memUtil = dp.DoubleValue()
		} else if metric.Name() == "process.memory.usage" {
			dp := metric.Sum().DataPoints().At(0)
			timestamp = dp.Timestamp()
			memUsage = dp.IntValue()
		}
	}

	memUtilPct := memUtil / 100

	addMetrics(metrics, dataset,
		metric{
			dataType:  Sum,
			name:      "system.process.num_threads",
			timestamp: timestamp,
			intValue:  &threads,
		},
		metric{
			dataType:    Gauge,
			name:        "system.process.memory.rss.pct",
			timestamp:   timestamp,
			doubleValue: &memUtilPct,
		},
		metric{
			dataType:  Sum,
			name:      "system.process.memory.rss.bytes",
			timestamp: timestamp,
			intValue:  &memUsage,
		},
	)

	return nil
}
