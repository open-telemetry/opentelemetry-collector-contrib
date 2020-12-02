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
	"go.opentelemetry.io/collector/consumer/pdata"
)

func convertToOTLPMetrics(prefix string, m ECSMetrics, r pdata.Resource, timestamp pdata.TimestampUnixNano) pdata.ResourceMetricsSlice {
	rms := pdata.NewResourceMetricsSlice()
	rms.Resize(1)
	rm := rms.At(0)

	r.CopyTo(rm.Resource())

	ilms := rm.InstrumentationLibraryMetrics()

	ilms.Append(intGauge(prefix+AttributeMemoryUsage, UnitBytes, int64(m.MemoryUsage), timestamp))
	ilms.Append(intGauge(prefix+AttributeMemoryMaxUsage, UnitBytes, int64(m.MemoryMaxUsage), timestamp))
	ilms.Append(intGauge(prefix+AttributeMemoryLimit, UnitBytes, int64(m.MemoryLimit), timestamp))
	ilms.Append(intGauge(prefix+AttributeMemoryUtilized, UnitMegaBytes, int64(m.MemoryUtilized), timestamp))
	ilms.Append(intGauge(prefix+AttributeMemoryReserved, UnitMegaBytes, int64(m.MemoryReserved), timestamp))

	ilms.Append(intSum(prefix+AttributeCPUTotalUsage, UnitNanoSecond, int64(m.CPUTotalUsage), timestamp))
	ilms.Append(intSum(prefix+AttributeCPUKernelModeUsage, UnitNanoSecond, int64(m.CPUUsageInKernelmode), timestamp))
	ilms.Append(intSum(prefix+AttributeCPUUserModeUsage, UnitNanoSecond, int64(m.CPUUsageInUserMode), timestamp))
	ilms.Append(intGauge(prefix+AttributeCPUCores, UnitCount, int64(m.NumOfCPUCores), timestamp))
	ilms.Append(intGauge(prefix+AttributeCPUOnlines, UnitCount, int64(m.CPUOnlineCpus), timestamp))
	ilms.Append(intSum(prefix+AttributeCPUSystemUsage, UnitNanoSecond, int64(m.SystemCPUUsage), timestamp))
	ilms.Append(doubleGauge(prefix+AttributeCPUUtilized, UnitPercent, m.CPUUtilized, timestamp))
	ilms.Append(doubleGauge(prefix+AttributeCPUReserved, UnitVCpu, m.CPUReserved, timestamp))
	ilms.Append(doubleGauge(prefix+AttributeCPUUsageInVCPU, UnitVCpu, m.CPUUsageInVCPU, timestamp))

	ilms.Append(doubleGauge(prefix+AttributeNetworkRateRx, UnitBytesPerSec, m.NetworkRateRxBytesPerSecond, timestamp))
	ilms.Append(doubleGauge(prefix+AttributeNetworkRateTx, UnitBytesPerSec, m.NetworkRateTxBytesPerSecond, timestamp))

	ilms.Append(intSum(prefix+AttributeNetworkRxBytes, UnitBytes, int64(m.NetworkRxBytes), timestamp))
	ilms.Append(intSum(prefix+AttributeNetworkRxPackets, UnitCount, int64(m.NetworkRxPackets), timestamp))
	ilms.Append(intSum(prefix+AttributeNetworkRxErrors, UnitCount, int64(m.NetworkRxErrors), timestamp))
	ilms.Append(intSum(prefix+AttributeNetworkRxDropped, UnitCount, int64(m.NetworkRxDropped), timestamp))
	ilms.Append(intSum(prefix+AttributeNetworkTxBytes, UnitBytes, int64(m.NetworkTxBytes), timestamp))
	ilms.Append(intSum(prefix+AttributeNetworkTxPackets, UnitCount, int64(m.NetworkTxPackets), timestamp))
	ilms.Append(intSum(prefix+AttributeNetworkTxErrors, UnitCount, int64(m.NetworkTxErrors), timestamp))
	ilms.Append(intSum(prefix+AttributeNetworkTxDropped, UnitCount, int64(m.NetworkTxDropped), timestamp))

	ilms.Append(intSum(prefix+AttributeStorageRead, UnitBytes, int64(m.StorageReadBytes), timestamp))
	ilms.Append(intSum(prefix+AttributeStorageWrite, UnitBytes, int64(m.StorageWriteBytes), timestamp))

	return rms
}

func intGauge(metricName string, unit string, value int64, ts pdata.TimestampUnixNano) pdata.InstrumentationLibraryMetrics {
	ilm := pdata.NewInstrumentationLibraryMetrics()

	metric := initMetric(ilm, metricName, unit)

	metric.SetDataType(pdata.MetricDataTypeIntGauge)
	intGauge := metric.IntGauge()

	updateIntDataPoint(intGauge.DataPoints(), value, ts)

	return ilm
}

func intSum(metricName string, unit string, value int64, ts pdata.TimestampUnixNano) pdata.InstrumentationLibraryMetrics {
	ilm := pdata.NewInstrumentationLibraryMetrics()

	metric := initMetric(ilm, metricName, unit)

	metric.SetDataType(pdata.MetricDataTypeIntSum)
	intSum := metric.IntSum()
	intSum.SetAggregationTemporality(pdata.AggregationTemporalityCumulative)

	updateIntDataPoint(intSum.DataPoints(), value, ts)

	return ilm
}

func doubleGauge(metricName string, unit string, value float64, ts pdata.TimestampUnixNano) pdata.InstrumentationLibraryMetrics {
	ilm := pdata.NewInstrumentationLibraryMetrics()

	metric := initMetric(ilm, metricName, unit)

	metric.SetDataType(pdata.MetricDataTypeDoubleGauge)
	doubleGauge := metric.DoubleGauge()
	dataPoints := doubleGauge.DataPoints()
	dataPoints.Resize(1)
	dataPoint := dataPoints.At(0)

	dataPoint.SetValue(value)
	dataPoint.SetTimestamp(ts)

	return ilm
}

func updateIntDataPoint(dataPoints pdata.IntDataPointSlice, value int64, ts pdata.TimestampUnixNano) {
	dataPoints.Resize(1)
	dataPoint := dataPoints.At(0)
	dataPoint.SetValue(value)
	dataPoint.SetTimestamp(ts)
}

func initMetric(ilm pdata.InstrumentationLibraryMetrics, name, unit string) pdata.Metric {
	ilm.Metrics().Resize(1)
	metric := ilm.Metrics().At(0)
	metric.SetName(name)
	metric.SetUnit(unit)

	return metric
}
