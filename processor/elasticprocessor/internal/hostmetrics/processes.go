package hostmetrics

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func addProcessesMetrics(metrics pmetric.MetricSlice, dataset string) error {
	var timestamp pcommon.Timestamp
	var total, value, idleProcesses, sleepingProcesses, stoppedProcesses, zombieProcesses, totalProcesses int64

	// iterate all metrics in the current scope and generate the additional Elastic system integration metrics
	for i := 0; i < metrics.Len(); i++ {
		metric := metrics.At(i)
		if metric.Name() == "system.processes.count" {
			dataPoints := metric.Sum().DataPoints()
			// iterate over the datapoints corresponding to different 'status' attributes
			for j := 0; j < dataPoints.Len(); j++ {
				dp := dataPoints.At(j)
				timestamp = dp.Timestamp()
				value = dp.IntValue()
				if status, ok := dp.Attributes().Get("status"); ok {
					switch status.Str() {
					case "idle":
						idleProcesses = value
						total += value
					case "sleeping":
						sleepingProcesses = value
						total += value
					case "stopped":
						stoppedProcesses = value
						total += value
					case "zombies":
						zombieProcesses = value
						total += value
					}
				}
			}
		}

	}
	totalProcesses = total

	addMetrics(metrics, dataset,
		metric{
			dataType:  Sum,
			name:      "system.process.summary.idle",
			timestamp: timestamp,
			intValue:  &idleProcesses,
		},
		metric{
			dataType:  Sum,
			name:      "system.process.summary.sleeping",
			timestamp: timestamp,
			intValue:  &sleepingProcesses,
		},
		metric{
			dataType:  Sum,
			name:      "system.process.summary.stopped",
			timestamp: timestamp,
			intValue:  &stoppedProcesses,
		},
		metric{
			dataType:  Sum,
			name:      "system.process.summary.zombie",
			timestamp: timestamp,
			intValue:  &zombieProcesses,
		},
		metric{
			dataType:  Sum,
			name:      "system.process.summary.total",
			timestamp: timestamp,
			intValue:  &totalProcesses,
		},
	)
	return nil
}
