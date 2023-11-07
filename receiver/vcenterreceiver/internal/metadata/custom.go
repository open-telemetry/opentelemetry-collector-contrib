// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metadata // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/vcenterreceiver/internal/metadata"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
)

// RecordVcenterVMNetworkThroughputDataPointWithoutObject adds a data point to vcenter.vm.network.throughput metric without an object metric attribute
func (mb *MetricsBuilder) RecordVcenterVMNetworkThroughputDataPointWithoutObject(ts pcommon.Timestamp, val int64, attr AttributeThroughputDirection) {
	mb.metricVcenterVMNetworkThroughput.recordDataPointWithoutObject(mb.startTime, ts, val, attr.String())
}

func (m *metricVcenterVMNetworkThroughput) recordDataPointWithoutObject(start pcommon.Timestamp, ts pcommon.Timestamp, val int64, throughputDirectionAttributeValue string) {
	if !m.config.Enabled {
		return
	}
	dp := m.data.Sum().DataPoints().AppendEmpty()
	dp.SetStartTimestamp(start)
	dp.SetTimestamp(ts)
	dp.SetIntValue(val)
	dp.Attributes().PutStr("direction", throughputDirectionAttributeValue)
}

// RecordVcenterVMNetworkUsageDataPointWithoutObject adds a data point to vcenter.vm.network.usage metric without an object metric attribute
func (mb *MetricsBuilder) RecordVcenterVMNetworkUsageDataPointWithoutObject(ts pcommon.Timestamp, val int64) {
	mb.metricVcenterVMNetworkUsage.recordDataPointWithoutObject(mb.startTime, ts, val)
}

func (m *metricVcenterVMNetworkUsage) recordDataPointWithoutObject(start pcommon.Timestamp, ts pcommon.Timestamp, val int64) {
	if !m.config.Enabled {
		return
	}
	dp := m.data.Sum().DataPoints().AppendEmpty()
	dp.SetStartTimestamp(start)
	dp.SetTimestamp(ts)
	dp.SetIntValue(val)
}

// RecordVcenterVMNetworkPacketCountDataPointWithoutObject adds a data point to vcenter.vm.network.packet.count metric without an object metric attribute
func (mb *MetricsBuilder) RecordVcenterVMNetworkPacketCountDataPointWithoutObject(ts pcommon.Timestamp, val int64, attr AttributeThroughputDirection) {
	mb.metricVcenterVMNetworkPacketCount.recordDataPointWithoutObject(mb.startTime, ts, val, attr.String())
}

func (m *metricVcenterVMNetworkPacketCount) recordDataPointWithoutObject(start pcommon.Timestamp, ts pcommon.Timestamp, val int64, throughputDirectionAttributeValue string) {
	if !m.config.Enabled {
		return
	}
	dp := m.data.Sum().DataPoints().AppendEmpty()
	dp.SetStartTimestamp(start)
	dp.SetTimestamp(ts)
	dp.SetIntValue(val)
	dp.Attributes().PutStr("direction", throughputDirectionAttributeValue)

}

// RecordVcenterVMDiskLatencyAvgDataPointWithoutObject adds a data point to vcenter.vm.disk.latency.avg metric without an object metric attribute
func (mb *MetricsBuilder) RecordVcenterVMDiskLatencyAvgDataPointWithoutObject(ts pcommon.Timestamp, val int64, diskDirection AttributeDiskDirection, diskType AttributeDiskType) {
	mb.metricVcenterVMDiskLatencyAvg.recordDataPointWithoutObject(mb.startTime, ts, val, diskDirection.String(), diskType.String())
}

func (m *metricVcenterVMDiskLatencyAvg) recordDataPointWithoutObject(start pcommon.Timestamp, ts pcommon.Timestamp, val int64, diskDirection string, diskType string) {
	if !m.config.Enabled {
		return
	}
	dp := m.data.Gauge().DataPoints().AppendEmpty()
	dp.SetStartTimestamp(start)
	dp.SetTimestamp(ts)
	dp.SetIntValue(val)
	dp.Attributes().PutStr("direction", diskDirection)
	dp.Attributes().PutStr("disk_type", diskType)
}

// RecordVcenterVMDiskLatencyMaxDataPointWithoutObject adds a data point to vcenter.vm.disk.latency.max metric without an object metric attribute
func (mb *MetricsBuilder) RecordVcenterVMDiskLatencyMaxDataPointWithoutObject(ts pcommon.Timestamp, val int64) {
	mb.metricVcenterVMDiskLatencyMax.recordDataPointWithoutObject(mb.startTime, ts, val)
}

func (m *metricVcenterVMDiskLatencyMax) recordDataPointWithoutObject(start pcommon.Timestamp, ts pcommon.Timestamp, val int64) {
	if !m.config.Enabled {
		return
	}
	dp := m.data.Gauge().DataPoints().AppendEmpty()
	dp.SetStartTimestamp(start)
	dp.SetTimestamp(ts)
	dp.SetIntValue(val)
}

// RecordVcenterHostNetworkUsageDataPointWithoutObject adds a data point to vcenter.host.network.usage metric without an object metric attribute
func (mb *MetricsBuilder) RecordVcenterHostNetworkUsageDataPointWithoutObject(ts pcommon.Timestamp, val int64) {
	mb.metricVcenterHostNetworkUsage.recordDataPointWithoutObject(mb.startTime, ts, val)
}

func (m *metricVcenterHostNetworkUsage) recordDataPointWithoutObject(start pcommon.Timestamp, ts pcommon.Timestamp, val int64) {
	if !m.config.Enabled {
		return
	}
	dp := m.data.Sum().DataPoints().AppendEmpty()
	dp.SetStartTimestamp(start)
	dp.SetTimestamp(ts)
	dp.SetIntValue(val)
}

// RecordVcenterHostNetworkThroughputDataPointWithoutObject adds a data point to vcenter.host.network.throughput metric without an object metric attribute
func (mb *MetricsBuilder) RecordVcenterHostNetworkThroughputDataPointWithoutObject(ts pcommon.Timestamp, val int64, attr AttributeThroughputDirection) {
	mb.metricVcenterHostNetworkThroughput.recordDataPointWithoutObject(mb.startTime, ts, val, attr.String())
}

func (m *metricVcenterHostNetworkThroughput) recordDataPointWithoutObject(start pcommon.Timestamp, ts pcommon.Timestamp, val int64, throughputDirectionAttributeValue string) {
	if !m.config.Enabled {
		return
	}
	dp := m.data.Sum().DataPoints().AppendEmpty()
	dp.SetStartTimestamp(start)
	dp.SetTimestamp(ts)
	dp.SetIntValue(val)
	dp.Attributes().PutStr("direction", throughputDirectionAttributeValue)
}

// RecordVcenterHostNetworkPacketCountDataPointWithoutObject adds a data point to vcenter.host.network.packet.count metric without an object metric attribute
func (mb *MetricsBuilder) RecordVcenterHostNetworkPacketCountDataPointWithoutObject(ts pcommon.Timestamp, val int64, attr AttributeThroughputDirection) {
	mb.metricVcenterHostNetworkPacketCount.recordDataPointWithoutObject(mb.startTime, ts, val, attr.String())
}

func (m *metricVcenterHostNetworkPacketCount) recordDataPointWithoutObject(start pcommon.Timestamp, ts pcommon.Timestamp, val int64, throughputDirectionAttributeValue string) {
	if !m.config.Enabled {
		return
	}
	dp := m.data.Sum().DataPoints().AppendEmpty()
	dp.SetStartTimestamp(start)
	dp.SetTimestamp(ts)
	dp.SetIntValue(val)
	dp.Attributes().PutStr("direction", throughputDirectionAttributeValue)
}

// RecordVcenterHostNetworkPacketErrorsDataPointWithoutObject adds a data point to vcenter.host.network.packet.errors metric without an object metric attribute
func (mb *MetricsBuilder) RecordVcenterHostNetworkPacketErrorsDataPointWithoutObject(ts pcommon.Timestamp, val int64, throughputDirection AttributeThroughputDirection) {
	mb.metricVcenterHostNetworkPacketErrors.recordDataPointWithoutObject(mb.startTime, ts, val, throughputDirection.String())
}

func (m *metricVcenterHostNetworkPacketErrors) recordDataPointWithoutObject(start pcommon.Timestamp, ts pcommon.Timestamp, val int64, throughputDirectionAttributeValue string) {
	if !m.config.Enabled {
		return
	}
	dp := m.data.Sum().DataPoints().AppendEmpty()
	dp.SetStartTimestamp(start)
	dp.SetTimestamp(ts)
	dp.SetIntValue(val)
	dp.Attributes().PutStr("direction", throughputDirectionAttributeValue)
}

// RecordVcenterHostDiskLatencyAvgDataPointWithoutObject adds a data point to vcenter.host.disk.latency.avg metric without an object metric attribute
func (mb *MetricsBuilder) RecordVcenterHostDiskLatencyAvgDataPointWithoutObject(ts pcommon.Timestamp, val int64, diskDirection AttributeDiskDirection) {
	mb.metricVcenterHostDiskLatencyAvg.recordDataPointWithoutObject(mb.startTime, ts, val, diskDirection.String())
}

func (m *metricVcenterHostDiskLatencyAvg) recordDataPointWithoutObject(start pcommon.Timestamp, ts pcommon.Timestamp, val int64, diskDirectionAttributeValue string) {
	if !m.config.Enabled {
		return
	}
	dp := m.data.Gauge().DataPoints().AppendEmpty()
	dp.SetStartTimestamp(start)
	dp.SetTimestamp(ts)
	dp.SetIntValue(val)
	dp.Attributes().PutStr("direction", diskDirectionAttributeValue)
}

// RecordVcenterHostDiskLatencyMaxDataPointWithoutObject adds a data point to vcenter.host.disk.latency.max metric without an object metric attribute
func (mb *MetricsBuilder) RecordVcenterHostDiskLatencyMaxDataPointWithoutObject(ts pcommon.Timestamp, val int64) {
	mb.metricVcenterHostDiskLatencyMax.recordDataPointWithoutObject(mb.startTime, ts, val)
}

func (m *metricVcenterHostDiskLatencyMax) recordDataPointWithoutObject(start pcommon.Timestamp, ts pcommon.Timestamp, val int64) {
	if !m.config.Enabled {
		return
	}
	dp := m.data.Gauge().DataPoints().AppendEmpty()
	dp.SetStartTimestamp(start)
	dp.SetTimestamp(ts)
	dp.SetIntValue(val)
}

// RecordVcenterHostDiskThroughputDataPointWithoutObject adds a data point to vcenter.host.disk.throughput metric without an object metric attribute
func (mb *MetricsBuilder) RecordVcenterHostDiskThroughputDataPointWithoutObject(ts pcommon.Timestamp, val int64, diskDirection AttributeDiskDirection) {
	mb.metricVcenterHostDiskThroughput.recordDataPointWithoutObject(mb.startTime, ts, val, diskDirection.String())
}

func (m *metricVcenterHostDiskThroughput) recordDataPointWithoutObject(start pcommon.Timestamp, ts pcommon.Timestamp, val int64, diskDirectionAttributeValue string) {
	if !m.config.Enabled {
		return
	}
	dp := m.data.Sum().DataPoints().AppendEmpty()
	dp.SetStartTimestamp(start)
	dp.SetTimestamp(ts)
	dp.SetIntValue(val)
	dp.Attributes().PutStr("direction", diskDirectionAttributeValue)
}
