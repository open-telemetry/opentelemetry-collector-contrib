package hostmetrics

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func addProcessMetrics(metrics pmetric.MetricSlice, dataset string) error {
	var timestamp pcommon.Timestamp
	var threads int64

	for i := 0; i < metrics.Len(); i++ {
		metric := metrics.At(i)
		if metric.Name() == "process.threads" {
			dp := metric.Sum().DataPoints().At(0)
			timestamp = dp.Timestamp()
			threads = dp.IntValue()
		}
	}

	addMetrics(metrics, dataset,
		metric{
			dataType:  Sum,
			name:      "system.process.num_threads",
			timestamp: timestamp,
			intValue:  &threads,
		},
	)

	return nil
}
