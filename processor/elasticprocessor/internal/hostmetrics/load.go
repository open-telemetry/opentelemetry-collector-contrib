package hostmetrics

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func transformLoadMetrics(metrics pmetric.MetricSlice) error {
	var timestamp, startTimestamp pcommon.Timestamp
	var l1, l5, l15 float64
	cpus := map[string]bool{}

	for i := 0; i < metrics.Len(); i++ {
		metric := metrics.At(i)
		if metric.Name() == "system.cpu.load_average.1m" {
			dp := metric.Gauge().DataPoints().At(0)
			l1 = dp.DoubleValue()

			// common values
			timestamp = dp.Timestamp()
			startTimestamp = dp.StartTimestamp()
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
	coresScaler := float64(cores)
	
	// add number of new metrics added below
	metrics.EnsureCapacity(metrics.Len() + 4)

	m := metrics.AppendEmpty()
	m.SetName("system.load.cores")
	dp := m.SetEmptySum().DataPoints().AppendEmpty()
	dp.SetTimestamp(timestamp)
	dp.SetStartTimestamp(startTimestamp)
	dp.SetIntValue(cores)

	m = metrics.AppendEmpty()
	m.SetName("system.load.norm.1")
	dp = m.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.SetTimestamp(timestamp)
	dp.SetStartTimestamp(startTimestamp)
	dp.SetDoubleValue(l1 / coresScaler)

	m = metrics.AppendEmpty()
	m.SetName("system.load.norm.5")
	dp = m.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.SetTimestamp(timestamp)
	dp.SetStartTimestamp(startTimestamp)
	dp.SetDoubleValue(l5 / coresScaler)

	m = metrics.AppendEmpty()
	m.SetName("system.load.norm.15")
	dp = m.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.SetTimestamp(timestamp)
	dp.SetStartTimestamp(startTimestamp)
	dp.SetDoubleValue(l15 / coresScaler)
	return nil
}
