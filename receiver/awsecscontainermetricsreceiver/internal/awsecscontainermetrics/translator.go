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
	conventions "go.opentelemetry.io/collector/model/semconv/v1.5.0"
)

func convertToOTLPMetrics(prefix string, m ECSMetrics, r pdata.Resource, timestamp pdata.Timestamp) pdata.Metrics {
	md := pdata.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	rm.SetSchemaUrl(conventions.SchemaURL)
	r.CopyTo(rm.Resource())

	ilms := rm.InstrumentationLibraryMetrics()

	appendIntGauge(prefix+attributeMemoryUsage, unitBytes, int64(m.MemoryUsage), timestamp, ilms.AppendEmpty())
	appendIntGauge(prefix+attributeMemoryMaxUsage, unitBytes, int64(m.MemoryMaxUsage), timestamp, ilms.AppendEmpty())
	appendIntGauge(prefix+attributeMemoryLimit, unitBytes, int64(m.MemoryLimit), timestamp, ilms.AppendEmpty())
	appendIntGauge(prefix+attributeMemoryUtilized, unitMegaBytes, int64(m.MemoryUtilized), timestamp, ilms.AppendEmpty())
	appendIntGauge(prefix+attributeMemoryReserved, unitMegaBytes, int64(m.MemoryReserved), timestamp, ilms.AppendEmpty())

	appendIntSum(prefix+attributeCPUTotalUsage, unitNanoSecond, int64(m.CPUTotalUsage), timestamp, ilms.AppendEmpty())
	appendIntSum(prefix+attributeCPUKernelModeUsage, unitNanoSecond, int64(m.CPUUsageInKernelmode), timestamp, ilms.AppendEmpty())
	appendIntSum(prefix+attributeCPUUserModeUsage, unitNanoSecond, int64(m.CPUUsageInUserMode), timestamp, ilms.AppendEmpty())
	appendIntGauge(prefix+attributeCPUCores, unitCount, int64(m.NumOfCPUCores), timestamp, ilms.AppendEmpty())
	appendIntGauge(prefix+attributeCPUOnlines, unitCount, int64(m.CPUOnlineCpus), timestamp, ilms.AppendEmpty())
	appendIntSum(prefix+attributeCPUSystemUsage, unitNanoSecond, int64(m.SystemCPUUsage), timestamp, ilms.AppendEmpty())
	appendDoubleGauge(prefix+attributeCPUUtilized, unitPercent, m.CPUUtilized, timestamp, ilms.AppendEmpty())
	appendDoubleGauge(prefix+attributeCPUReserved, unitVCpu, m.CPUReserved, timestamp, ilms.AppendEmpty())
	appendDoubleGauge(prefix+attributeCPUUsageInVCPU, unitVCpu, m.CPUUsageInVCPU, timestamp, ilms.AppendEmpty())

	appendDoubleGauge(prefix+attributeNetworkRateRx, unitBytesPerSec, m.NetworkRateRxBytesPerSecond, timestamp, ilms.AppendEmpty())
	appendDoubleGauge(prefix+attributeNetworkRateTx, unitBytesPerSec, m.NetworkRateTxBytesPerSecond, timestamp, ilms.AppendEmpty())

	appendIntSum(prefix+attributeNetworkRxBytes, unitBytes, int64(m.NetworkRxBytes), timestamp, ilms.AppendEmpty())
	appendIntSum(prefix+attributeNetworkRxPackets, unitCount, int64(m.NetworkRxPackets), timestamp, ilms.AppendEmpty())
	appendIntSum(prefix+attributeNetworkRxErrors, unitCount, int64(m.NetworkRxErrors), timestamp, ilms.AppendEmpty())
	appendIntSum(prefix+attributeNetworkRxDropped, unitCount, int64(m.NetworkRxDropped), timestamp, ilms.AppendEmpty())
	appendIntSum(prefix+attributeNetworkTxBytes, unitBytes, int64(m.NetworkTxBytes), timestamp, ilms.AppendEmpty())
	appendIntSum(prefix+attributeNetworkTxPackets, unitCount, int64(m.NetworkTxPackets), timestamp, ilms.AppendEmpty())
	appendIntSum(prefix+attributeNetworkTxErrors, unitCount, int64(m.NetworkTxErrors), timestamp, ilms.AppendEmpty())
	appendIntSum(prefix+attributeNetworkTxDropped, unitCount, int64(m.NetworkTxDropped), timestamp, ilms.AppendEmpty())

	appendIntSum(prefix+attributeStorageRead, unitBytes, int64(m.StorageReadBytes), timestamp, ilms.AppendEmpty())
	appendIntSum(prefix+attributeStorageWrite, unitBytes, int64(m.StorageWriteBytes), timestamp, ilms.AppendEmpty())

	return md
}

func convertStoppedContainerDataToOTMetrics(prefix string, containerResource pdata.Resource, timestamp pdata.Timestamp, duration float64) pdata.Metrics {
	md := pdata.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	containerResource.CopyTo(rm.Resource())
	ilms := rm.InstrumentationLibraryMetrics()

	appendDoubleGauge(prefix+attributeDuration, unitSecond, duration, timestamp, ilms.AppendEmpty())

	return md
}

func appendIntGauge(metricName string, unit string, value int64, ts pdata.Timestamp, ilm pdata.InstrumentationLibraryMetrics) {
	metric := appendMetric(ilm, metricName, unit)

	metric.SetDataType(pdata.MetricDataTypeGauge)
	intGauge := metric.Gauge()

	appendIntDataPoint(intGauge.DataPoints(), value, ts)
}

func appendIntSum(metricName string, unit string, value int64, ts pdata.Timestamp, ilm pdata.InstrumentationLibraryMetrics) {
	metric := appendMetric(ilm, metricName, unit)

	metric.SetDataType(pdata.MetricDataTypeSum)
	intSum := metric.Sum()
	intSum.SetAggregationTemporality(pdata.MetricAggregationTemporalityCumulative)

	appendIntDataPoint(intSum.DataPoints(), value, ts)
}

func appendDoubleGauge(metricName string, unit string, value float64, ts pdata.Timestamp, ilm pdata.InstrumentationLibraryMetrics) {
	metric := appendMetric(ilm, metricName, unit)
	metric.SetDataType(pdata.MetricDataTypeGauge)
	doubleGauge := metric.Gauge()
	dataPoint := doubleGauge.DataPoints().AppendEmpty()
	dataPoint.SetDoubleVal(value)
	dataPoint.SetTimestamp(ts)
}

func appendIntDataPoint(dataPoints pdata.NumberDataPointSlice, value int64, ts pdata.Timestamp) {
	dataPoint := dataPoints.AppendEmpty()
	dataPoint.SetIntVal(value)
	dataPoint.SetTimestamp(ts)
}

func appendMetric(ilm pdata.InstrumentationLibraryMetrics, name, unit string) pdata.Metric {
	metric := ilm.Metrics().AppendEmpty()
	metric.SetName(name)
	metric.SetUnit(unit)

	return metric
}
