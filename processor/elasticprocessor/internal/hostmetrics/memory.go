package hostmetrics

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func transformMemoryMetrics(metrics pmetric.MetricSlice) error {
	var timestamp, startTimestamp pcommon.Timestamp
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
				startTimestamp = dp.StartTimestamp()

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

	// add number of new metrics added below
	metrics.EnsureCapacity(metrics.Len() + 8)

	m := metrics.AppendEmpty()
	m.SetName("system.memory.total")
	dp := m.SetEmptySum().DataPoints().AppendEmpty()
	dp.SetTimestamp(timestamp)
	dp.SetStartTimestamp(startTimestamp)
	dp.SetIntValue(total)

	m = metrics.AppendEmpty()
	m.SetName("system.memory.free")
	dp = m.SetEmptySum().DataPoints().AppendEmpty()
	dp.SetTimestamp(timestamp)
	dp.SetStartTimestamp(startTimestamp)
	dp.SetIntValue(free)

	m = metrics.AppendEmpty()
	m.SetName("system.memory.cached")
	dp = m.SetEmptySum().DataPoints().AppendEmpty()
	dp.SetTimestamp(timestamp)
	dp.SetStartTimestamp(startTimestamp)
	dp.SetIntValue(cached)

	m = metrics.AppendEmpty()
	m.SetName("system.memory.used.bytes")
	dp = m.SetEmptySum().DataPoints().AppendEmpty()
	dp.SetTimestamp(timestamp)
	dp.SetStartTimestamp(startTimestamp)
	dp.SetIntValue(usedBytes)

	m = metrics.AppendEmpty()
	m.SetName("system.memory.actual.used.bytes")
	dp = m.SetEmptySum().DataPoints().AppendEmpty()
	dp.SetTimestamp(timestamp)
	dp.SetStartTimestamp(startTimestamp)
	dp.SetIntValue(actualUsedBytes)

	m = metrics.AppendEmpty()
	m.SetName("system.memory.actual.free")
	dp = m.SetEmptySum().DataPoints().AppendEmpty()
	dp.SetTimestamp(timestamp)
	dp.SetStartTimestamp(startTimestamp)
	dp.SetIntValue(actualFree)

	m = metrics.AppendEmpty()
	m.SetName("system.memory.used.pct")
	dp = m.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.SetTimestamp(timestamp)
	dp.SetStartTimestamp(startTimestamp)
	dp.SetDoubleValue(usedPercent)

	m = metrics.AppendEmpty()
	m.SetName("system.memory.actual.used.pct")
	dp = m.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.SetTimestamp(timestamp)
	dp.SetStartTimestamp(startTimestamp)
	dp.SetDoubleValue(actualUsedPercent)

	return nil
}
