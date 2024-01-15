package hostmetrics

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func transformCPUMetrics(metrics pmetric.MetricSlice) error {
	var timestamp, startTimestamp pcommon.Timestamp
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
			startTimestamp = dp.StartTimestamp()
		} else if metric.Name() == "system.cpu.utilization" {
			dataPoints := metric.Gauge().DataPoints()
			for j := 0; j < dataPoints.Len(); j++ {
				dataPoint := dataPoints.At(j)
				value := dataPoint.DoubleValue()

				if state, ok := dataPoint.Attributes().Get("state"); ok {
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

	numCoresScaler := float64(numCores)

	// add number of new metrics added below
	metrics.EnsureCapacity(metrics.Len() + 19)

	m := metrics.AppendEmpty()
	m.SetName("system.cpu.cores")
	dp := m.SetEmptySum().DataPoints().AppendEmpty()
	dp.SetTimestamp(timestamp)
	dp.SetStartTimestamp(startTimestamp)
	dp.SetIntValue(numCores)

	m = metrics.AppendEmpty()
	m.SetName("system.cpu.total.pct")
	dp = m.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.SetTimestamp(timestamp)
	dp.SetStartTimestamp(startTimestamp)
	dp.SetDoubleValue(totalPercent)

	m = metrics.AppendEmpty()
	m.SetName("system.cpu.total.norm.pct")
	dp = m.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.SetTimestamp(timestamp)
	dp.SetStartTimestamp(startTimestamp)
	dp.SetDoubleValue(totalPercent / numCoresScaler)

	m = metrics.AppendEmpty()
	m.SetName("system.cpu.idle.pct")
	dp = m.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.SetTimestamp(timestamp)
	dp.SetStartTimestamp(startTimestamp)
	dp.SetDoubleValue(idlePercent)

	m = metrics.AppendEmpty()
	m.SetName("system.cpu.idle.norm.pct")
	dp = m.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.SetTimestamp(timestamp)
	dp.SetStartTimestamp(startTimestamp)
	dp.SetDoubleValue(idlePercent / numCoresScaler)

	m = metrics.AppendEmpty()
	m.SetName("system.cpu.system.pct")
	dp = m.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.SetTimestamp(timestamp)
	dp.SetStartTimestamp(startTimestamp)
	dp.SetDoubleValue(systemPercent)

	m = metrics.AppendEmpty()
	m.SetName("system.cpu.system.norm.pct")
	dp = m.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.SetTimestamp(timestamp)
	dp.SetStartTimestamp(startTimestamp)
	dp.SetDoubleValue(systemPercent / numCoresScaler)

	m = metrics.AppendEmpty()
	m.SetName("system.cpu.user.pct")
	dp = m.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.SetTimestamp(timestamp)
	dp.SetStartTimestamp(startTimestamp)
	dp.SetDoubleValue(userPercent)

	m = metrics.AppendEmpty()
	m.SetName("system.cpu.user.norm.pct")
	dp = m.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.SetTimestamp(timestamp)
	dp.SetStartTimestamp(startTimestamp)
	dp.SetDoubleValue(userPercent / numCoresScaler)

	m = metrics.AppendEmpty()
	m.SetName("system.cpu.steal.pct")
	dp = m.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.SetTimestamp(timestamp)
	dp.SetStartTimestamp(startTimestamp)
	dp.SetDoubleValue(stealPercent)

	m = metrics.AppendEmpty()
	m.SetName("system.cpu.steal.norm.pct")
	dp = m.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.SetTimestamp(timestamp)
	dp.SetStartTimestamp(startTimestamp)
	dp.SetDoubleValue(stealPercent / numCoresScaler)

	m = metrics.AppendEmpty()
	m.SetName("system.cpu.wait.pct")
	dp = m.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.SetTimestamp(timestamp)
	dp.SetStartTimestamp(startTimestamp)
	dp.SetDoubleValue(iowaitPercent)

	m = metrics.AppendEmpty()
	m.SetName("system.cpu.wait.norm.pct")
	dp = m.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.SetTimestamp(timestamp)
	dp.SetStartTimestamp(startTimestamp)
	dp.SetDoubleValue(iowaitPercent / numCoresScaler)

	m = metrics.AppendEmpty()
	m.SetName("system.cpu.nice.pct")
	dp = m.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.SetTimestamp(timestamp)
	dp.SetStartTimestamp(startTimestamp)
	dp.SetDoubleValue(nicePercent)

	m = metrics.AppendEmpty()
	m.SetName("system.cpu.nice.norm.pct")
	dp = m.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.SetTimestamp(timestamp)
	dp.SetStartTimestamp(startTimestamp)
	dp.SetDoubleValue(nicePercent / numCoresScaler)

	m = metrics.AppendEmpty()
	m.SetName("system.cpu.interrupt.pct")
	dp = m.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.SetTimestamp(timestamp)
	dp.SetStartTimestamp(startTimestamp)
	dp.SetDoubleValue(irqPercent)

	m = metrics.AppendEmpty()
	m.SetName("system.cpu.interrupt.norm.pct")
	dp = m.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.SetTimestamp(timestamp)
	dp.SetStartTimestamp(startTimestamp)
	dp.SetDoubleValue(irqPercent / numCoresScaler)

	m = metrics.AppendEmpty()
	m.SetName("system.cpu.softirq.pct")
	dp = m.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.SetTimestamp(timestamp)
	dp.SetStartTimestamp(startTimestamp)
	dp.SetDoubleValue(softirqPercent)

	m = metrics.AppendEmpty()
	m.SetName("system.cpu.softirq.norm.pct")
	dp = m.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.SetTimestamp(timestamp)
	dp.SetStartTimestamp(startTimestamp)
	dp.SetDoubleValue(softirqPercent / numCoresScaler)

	return nil
}
