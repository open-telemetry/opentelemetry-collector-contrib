// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsecscontainermetrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsecscontainermetricsreceiver/internal/awsecscontainermetrics"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	conventions "go.opentelemetry.io/otel/semconv/v1.43.0"
)

func convertToOTLPMetrics(prefix string, m ECSMetrics, r pcommon.Resource, timestamp pcommon.Timestamp, startTimestamp pcommon.Timestamp) pmetric.Metrics {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	rm.SetSchemaUrl(conventions.SchemaURL)
	r.CopyTo(rm.Resource())

	ilms := rm.ScopeMetrics()

	appendIntGauge(prefix+attributeMemoryUsage, unitBytes, int64(m.MemoryUsage), timestamp, ilms.AppendEmpty())
	appendIntGauge(prefix+attributeMemoryMaxUsage, unitBytes, int64(m.MemoryMaxUsage), timestamp, ilms.AppendEmpty())
	appendIntGauge(prefix+attributeMemoryLimit, unitBytes, int64(m.MemoryLimit), timestamp, ilms.AppendEmpty())
	appendIntGauge(prefix+attributeMemoryUtilized, unitMiB, int64(m.MemoryUtilized), timestamp, ilms.AppendEmpty())
	appendIntGauge(prefix+attributeMemoryReserved, unitMiB, int64(m.MemoryReserved), timestamp, ilms.AppendEmpty())

	appendIntSum(prefix+attributeCPUTotalUsage, unitNanoSecond, int64(m.CPUTotalUsage), timestamp, startTimestamp, ilms.AppendEmpty())
	appendIntSum(prefix+attributeCPUKernelModeUsage, unitNanoSecond, int64(m.CPUUsageInKernelmode), timestamp, startTimestamp, ilms.AppendEmpty())
	appendIntSum(prefix+attributeCPUUserModeUsage, unitNanoSecond, int64(m.CPUUsageInUserMode), timestamp, startTimestamp, ilms.AppendEmpty())
	appendIntGauge(prefix+attributeCPUCores, unitCount, int64(m.NumOfCPUCores), timestamp, ilms.AppendEmpty())
	appendIntGauge(prefix+attributeCPUOnlines, unitCount, int64(m.CPUOnlineCpus), timestamp, ilms.AppendEmpty())
	appendIntSum(prefix+attributeCPUSystemUsage, unitNanoSecond, int64(m.SystemCPUUsage), timestamp, startTimestamp, ilms.AppendEmpty())
	appendDoubleGauge(prefix+attributeCPUUtilized, unitNone, m.CPUUtilized, timestamp, ilms.AppendEmpty())
	appendDoubleGauge(prefix+attributeCPUReserved, unitNone, m.CPUReserved, timestamp, ilms.AppendEmpty())
	appendDoubleGauge(prefix+attributeCPUUsageInVCPU, unitVCpu, m.CPUUsageInVCPU, timestamp, ilms.AppendEmpty())

	appendDoubleGauge(prefix+attributeNetworkRateRx, unitBytesPerSec, m.NetworkRateRxBytesPerSecond, timestamp, ilms.AppendEmpty())
	appendDoubleGauge(prefix+attributeNetworkRateTx, unitBytesPerSec, m.NetworkRateTxBytesPerSecond, timestamp, ilms.AppendEmpty())

	appendIntSum(prefix+attributeNetworkRxBytes, unitBytes, int64(m.NetworkRxBytes), timestamp, startTimestamp, ilms.AppendEmpty())
	appendIntSum(prefix+attributeNetworkRxPackets, unitCount, int64(m.NetworkRxPackets), timestamp, startTimestamp, ilms.AppendEmpty())
	appendIntSum(prefix+attributeNetworkRxErrors, unitCount, int64(m.NetworkRxErrors), timestamp, startTimestamp, ilms.AppendEmpty())
	appendIntSum(prefix+attributeNetworkRxDropped, unitCount, int64(m.NetworkRxDropped), timestamp, startTimestamp, ilms.AppendEmpty())
	appendIntSum(prefix+attributeNetworkTxBytes, unitBytes, int64(m.NetworkTxBytes), timestamp, startTimestamp, ilms.AppendEmpty())
	appendIntSum(prefix+attributeNetworkTxPackets, unitCount, int64(m.NetworkTxPackets), timestamp, startTimestamp, ilms.AppendEmpty())
	appendIntSum(prefix+attributeNetworkTxErrors, unitCount, int64(m.NetworkTxErrors), timestamp, startTimestamp, ilms.AppendEmpty())
	appendIntSum(prefix+attributeNetworkTxDropped, unitCount, int64(m.NetworkTxDropped), timestamp, startTimestamp, ilms.AppendEmpty())

	appendIntSum(prefix+attributeStorageRead, unitBytes, int64(m.StorageReadBytes), timestamp, startTimestamp, ilms.AppendEmpty())
	appendIntSum(prefix+attributeStorageWrite, unitBytes, int64(m.StorageWriteBytes), timestamp, startTimestamp, ilms.AppendEmpty())

	// Ephemeral storage metrics are only available at the task level.
	// They represent the shared ephemeral storage for the entire Fargate task.
	if prefix == taskPrefix {
		appendIntGauge(prefix+attributeEphemeralStorageUtilized, unitMiB, m.EphemeralStorageUtilized, timestamp, ilms.AppendEmpty())
		appendIntGauge(prefix+attributeEphemeralStorageReserved, unitMiB, m.EphemeralStorageReserved, timestamp, ilms.AppendEmpty())
	}

	return md
}

func convertStoppedContainerDataToOTMetrics(prefix string, containerResource pcommon.Resource, timestamp pcommon.Timestamp, duration float64) pmetric.Metrics {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	containerResource.CopyTo(rm.Resource())
	ilms := rm.ScopeMetrics()

	appendDoubleGauge(prefix+attributeDuration, unitSecond, duration, timestamp, ilms.AppendEmpty())

	return md
}

func appendIntGauge(metricName, unit string, value int64, ts pcommon.Timestamp, ilm pmetric.ScopeMetrics) {
	metric := appendMetric(ilm, metricName, unit)

	intGauge := metric.SetEmptyGauge()

	appendIntDataPoint(intGauge.DataPoints(), value, ts)
}

func appendIntSum(metricName, unit string, value int64, ts, startTs pcommon.Timestamp, ilm pmetric.ScopeMetrics) {
	metric := appendMetric(ilm, metricName, unit)

	intSum := metric.SetEmptySum()
	intSum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
	intSum.SetIsMonotonic(true)

	dataPoint := intSum.DataPoints().AppendEmpty()
	dataPoint.SetIntValue(value)
	dataPoint.SetTimestamp(ts)
	if startTs != 0 {
		dataPoint.SetStartTimestamp(startTs)
	}
}

func appendDoubleGauge(metricName, unit string, value float64, ts pcommon.Timestamp, ilm pmetric.ScopeMetrics) {
	metric := appendMetric(ilm, metricName, unit)
	doubleGauge := metric.SetEmptyGauge()
	dataPoint := doubleGauge.DataPoints().AppendEmpty()
	dataPoint.SetDoubleValue(value)
	dataPoint.SetTimestamp(ts)
}

func appendIntDataPoint(dataPoints pmetric.NumberDataPointSlice, value int64, ts pcommon.Timestamp) {
	dataPoint := dataPoints.AppendEmpty()
	dataPoint.SetIntValue(value)
	dataPoint.SetTimestamp(ts)
}

func appendMetric(ilm pmetric.ScopeMetrics, name, unit string) pmetric.Metric {
	metric := ilm.Metrics().AppendEmpty()
	metric.SetName(name)
	metric.SetUnit(unit)

	return metric
}
