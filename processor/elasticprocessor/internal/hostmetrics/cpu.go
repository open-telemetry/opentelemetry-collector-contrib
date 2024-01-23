package hostmetrics

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func addCPUMetrics(metrics pmetric.MetricSlice) error {
	var timestamp pcommon.Timestamp
	var numCores int64
	var totalPercent, idlePercent, systemPercent, userPercent, stealPercent,
		iowaitPercent, nicePercent, irqPercent, softirqPercent float64

	// iterate all metrics in the current scope and generate the additional Elastic system integration metrics
	for i := 0; i < metrics.Len(); i++ {
		metric := metrics.At(i)
		if metric.Name() == "system.cpu.logical.count" {
			dp := metric.Sum().DataPoints().At(0)
			numCores = dp.IntValue()
			timestamp = dp.Timestamp()
		} else if metric.Name() == "system.cpu.utilization" {
			dataPoints := metric.Gauge().DataPoints()
			for j := 0; j < dataPoints.Len(); j++ {
				dp := dataPoints.At(j)
				value := dp.DoubleValue()

				if state, ok := dp.Attributes().Get("state"); ok {
					switch state.Str() {
					case "idle":
						idlePercent += value
					case "system":
						systemPercent += value
						totalPercent += value
					case "user":
						userPercent += value
						totalPercent += value
					case "steal":
						stealPercent += value
						totalPercent += value
					case "wait":
						iowaitPercent += value
						totalPercent += value
					case "nice":
						nicePercent += value
						totalPercent += value
					case "interrupt":
						irqPercent += value
						totalPercent += value
					case "softirq":
						softirqPercent += value
						totalPercent += value
					}
				}
			}
		}
	}

	totalNorm := totalPercent / float64(numCores)
	idleNorm := idlePercent / float64(numCores)
	systemNorm := systemPercent / float64(numCores)
	userNorm := userPercent / float64(numCores)
	stealNorm := stealPercent / float64(numCores)
	iowaitNorm := iowaitPercent / float64(numCores)
	niceNorm := nicePercent / float64(numCores)
	irqNorm := irqPercent / float64(numCores)
	softirqNorm := softirqPercent / float64(numCores)

	addMetrics(metrics,
		metric{
			dataType:  Sum,
			name:      "system.cpu.cores",
			timestamp: timestamp,
			intValue:  &numCores,
		},
		metric{
			dataType:    Gauge,
			name:        "system.cpu.total.pct",
			timestamp:   timestamp,
			doubleValue: &totalPercent,
		},
		metric{
			dataType:    Gauge,
			name:        "system.cpu.idle.pct",
			timestamp:   timestamp,
			doubleValue: &idlePercent,
		},
		metric{
			dataType:    Gauge,
			name:        "system.cpu.system.pct",
			timestamp:   timestamp,
			doubleValue: &systemPercent,
		},
		metric{
			dataType:    Gauge,
			name:        "system.cpu.user.pct",
			timestamp:   timestamp,
			doubleValue: &userPercent,
		},
		metric{
			dataType:    Gauge,
			name:        "system.cpu.steal.pct",
			timestamp:   timestamp,
			doubleValue: &stealPercent,
		},
		metric{
			dataType:    Gauge,
			name:        "system.cpu.wait.pct",
			timestamp:   timestamp,
			doubleValue: &iowaitPercent,
		},
		metric{
			dataType:    Gauge,
			name:        "system.cpu.nice.pct",
			timestamp:   timestamp,
			doubleValue: &nicePercent,
		},
		metric{
			dataType:    Gauge,
			name:        "system.cpu.irq.pct",
			timestamp:   timestamp,
			doubleValue: &irqPercent,
		},
		metric{
			dataType:    Gauge,
			name:        "system.cpu.softirq.pct",
			timestamp:   timestamp,
			doubleValue: &softirqPercent,
		},
		metric{
			dataType:    Gauge,
			name:        "system.cpu.total.norm.pct",
			timestamp:   timestamp,
			doubleValue: &totalNorm,
		},
		metric{
			dataType:    Gauge,
			name:        "system.cpu.idle.norm.pct",
			timestamp:   timestamp,
			doubleValue: &idleNorm,
		},
		metric{
			dataType:    Gauge,
			name:        "system.cpu.system.norm.pct",
			timestamp:   timestamp,
			doubleValue: &systemNorm,
		},
		metric{
			dataType:    Gauge,
			name:        "system.cpu.user.norm.pct",
			timestamp:   timestamp,
			doubleValue: &userNorm,
		},
		metric{
			dataType:    Gauge,
			name:        "system.cpu.steal.norm.pct",
			timestamp:   timestamp,
			doubleValue: &stealNorm,
		},
		metric{
			dataType:    Gauge,
			name:        "system.cpu.wait.norm.pct",
			timestamp:   timestamp,
			doubleValue: &iowaitNorm,
		},
		metric{
			dataType:    Gauge,
			name:        "system.cpu.nice.norm.pct",
			timestamp:   timestamp,
			doubleValue: &niceNorm,
		},
		metric{
			dataType:    Gauge,
			name:        "system.cpu.irq.norm.pct",
			timestamp:   timestamp,
			doubleValue: &irqNorm,
		},
		metric{
			dataType:    Gauge,
			name:        "system.cpu.softirq.norm.pct",
			timestamp:   timestamp,
			doubleValue: &softirqNorm,
		},
	)

	return nil
}
