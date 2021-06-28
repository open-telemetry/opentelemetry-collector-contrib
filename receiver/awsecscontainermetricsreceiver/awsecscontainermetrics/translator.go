// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package awsecscontainermetrics

import (
	"go.opentelemetry.io/collector/model/pdata"
)

func convertToOTLPMetrics(prefix string, m ECSMetrics, r pdata.Resource, timestamp pdata.Timestamp) pdata.Metrics {
	md := pdata.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	r.CopyTo(rm.Resource())

	ilms := rm.InstrumentationLibraryMetrics()

	appendIntGauge(prefix+AttributeMemoryUsage, UnitBytes, int64(m.MemoryUsage), timestamp, ilms.AppendEmpty())
	appendIntGauge(prefix+AttributeMemoryMaxUsage, UnitBytes, int64(m.MemoryMaxUsage), timestamp, ilms.AppendEmpty())
	appendIntGauge(prefix+AttributeMemoryLimit, UnitBytes, int64(m.MemoryLimit), timestamp, ilms.AppendEmpty())
	appendIntGauge(prefix+AttributeMemoryUtilized, UnitMegaBytes, int64(m.MemoryUtilized), timestamp, ilms.AppendEmpty())
	appendIntGauge(prefix+AttributeMemoryReserved, UnitMegaBytes, int64(m.MemoryReserved), timestamp, ilms.AppendEmpty())

	appendIntSum(prefix+AttributeCPUTotalUsage, UnitNanoSecond, int64(m.CPUTotalUsage), timestamp, ilms.AppendEmpty())
	appendIntSum(prefix+AttributeCPUKernelModeUsage, UnitNanoSecond, int64(m.CPUUsageInKernelmode), timestamp, ilms.AppendEmpty())
	appendIntSum(prefix+AttributeCPUUserModeUsage, UnitNanoSecond, int64(m.CPUUsageInUserMode), timestamp, ilms.AppendEmpty())
	appendIntGauge(prefix+AttributeCPUCores, UnitCount, int64(m.NumOfCPUCores), timestamp, ilms.AppendEmpty())
	appendIntGauge(prefix+AttributeCPUOnlines, UnitCount, int64(m.CPUOnlineCpus), timestamp, ilms.AppendEmpty())
	appendIntSum(prefix+AttributeCPUSystemUsage, UnitNanoSecond, int64(m.SystemCPUUsage), timestamp, ilms.AppendEmpty())
	appendDoubleGauge(prefix+AttributeCPUUtilized, UnitPercent, m.CPUUtilized, timestamp, ilms.AppendEmpty())
	appendDoubleGauge(prefix+AttributeCPUReserved, UnitVCpu, m.CPUReserved, timestamp, ilms.AppendEmpty())
	appendDoubleGauge(prefix+AttributeCPUUsageInVCPU, UnitVCpu, m.CPUUsageInVCPU, timestamp, ilms.AppendEmpty())

	appendDoubleGauge(prefix+AttributeNetworkRateRx, UnitBytesPerSec, m.NetworkRateRxBytesPerSecond, timestamp, ilms.AppendEmpty())
	appendDoubleGauge(prefix+AttributeNetworkRateTx, UnitBytesPerSec, m.NetworkRateTxBytesPerSecond, timestamp, ilms.AppendEmpty())

	appendIntSum(prefix+AttributeNetworkRxBytes, UnitBytes, int64(m.NetworkRxBytes), timestamp, ilms.AppendEmpty())
	appendIntSum(prefix+AttributeNetworkRxPackets, UnitCount, int64(m.NetworkRxPackets), timestamp, ilms.AppendEmpty())
	appendIntSum(prefix+AttributeNetworkRxErrors, UnitCount, int64(m.NetworkRxErrors), timestamp, ilms.AppendEmpty())
	appendIntSum(prefix+AttributeNetworkRxDropped, UnitCount, int64(m.NetworkRxDropped), timestamp, ilms.AppendEmpty())
	appendIntSum(prefix+AttributeNetworkTxBytes, UnitBytes, int64(m.NetworkTxBytes), timestamp, ilms.AppendEmpty())
	appendIntSum(prefix+AttributeNetworkTxPackets, UnitCount, int64(m.NetworkTxPackets), timestamp, ilms.AppendEmpty())
	appendIntSum(prefix+AttributeNetworkTxErrors, UnitCount, int64(m.NetworkTxErrors), timestamp, ilms.AppendEmpty())
	appendIntSum(prefix+AttributeNetworkTxDropped, UnitCount, int64(m.NetworkTxDropped), timestamp, ilms.AppendEmpty())

	appendIntSum(prefix+AttributeStorageRead, UnitBytes, int64(m.StorageReadBytes), timestamp, ilms.AppendEmpty())
	appendIntSum(prefix+AttributeStorageWrite, UnitBytes, int64(m.StorageWriteBytes), timestamp, ilms.AppendEmpty())

	return md
}

func convertStoppedContainerDataToOTMetrics(prefix string, containerResource pdata.Resource, timestamp pdata.Timestamp, duration float64) pdata.Metrics {
	md := pdata.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	containerResource.CopyTo(rm.Resource())
	ilms := rm.InstrumentationLibraryMetrics()

	appendDoubleGauge(prefix+AttributeDuration, UnitSecond, duration, timestamp, ilms.AppendEmpty())

	return md
}

func appendIntGauge(metricName string, unit string, value int64, ts pdata.Timestamp, ilm pdata.InstrumentationLibraryMetrics) {
	metric := appendMetric(ilm, metricName, unit)

	metric.SetDataType(pdata.MetricDataTypeIntGauge)
	intGauge := metric.IntGauge()

	appendIntDataPoint(intGauge.DataPoints(), value, ts)
}

func appendIntSum(metricName string, unit string, value int64, ts pdata.Timestamp, ilm pdata.InstrumentationLibraryMetrics) {
	metric := appendMetric(ilm, metricName, unit)

	metric.SetDataType(pdata.MetricDataTypeIntSum)
	intSum := metric.IntSum()
	intSum.SetAggregationTemporality(pdata.AggregationTemporalityCumulative)

	appendIntDataPoint(intSum.DataPoints(), value, ts)
}

func appendDoubleGauge(metricName string, unit string, value float64, ts pdata.Timestamp, ilm pdata.InstrumentationLibraryMetrics) {
	metric := appendMetric(ilm, metricName, unit)
	metric.SetDataType(pdata.MetricDataTypeDoubleGauge)
	doubleGauge := metric.DoubleGauge()
	dataPoint := doubleGauge.DataPoints().AppendEmpty()
	dataPoint.SetValue(value)
	dataPoint.SetTimestamp(ts)
}

func appendIntDataPoint(dataPoints pdata.IntDataPointSlice, value int64, ts pdata.Timestamp) {
	dataPoint := dataPoints.AppendEmpty()
	dataPoint.SetValue(value)
	dataPoint.SetTimestamp(ts)
}

func appendMetric(ilm pdata.InstrumentationLibraryMetrics, name, unit string) pdata.Metric {
	metric := ilm.Metrics().AppendEmpty()
	metric.SetName(name)
	metric.SetUnit(unit)

	return metric
}
