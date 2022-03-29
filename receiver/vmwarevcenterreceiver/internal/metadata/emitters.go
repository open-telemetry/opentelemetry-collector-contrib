// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package metadata

import "go.opentelemetry.io/collector/model/pdata"

func (mb *MetricsBuilder) EmitVM(metrics pdata.MetricSlice) {
	mb.metricVcenterVMCPUUtilization.emit(metrics)
	mb.metricVcenterVMDiskLatency.emit(metrics)
	mb.metricVcenterVMDiskThroughput.emit(metrics)
	mb.metricVcenterVMDiskUsage.emit(metrics)
	mb.metricVcenterVMDiskLatency.emit(metrics)
	mb.metricVcenterVMMemoryBallooned.emit(metrics)
	mb.metricVcenterVMMemoryUsage.emit(metrics)
	mb.metricVcenterVMNetworkPackets.emit(metrics)
	mb.metricVcenterVMNetworkThroughput.emit(metrics)
	mb.metricVcenterVMVsanLatencyAvg.emit(metrics)
	mb.metricVcenterVMVsanOperations.emit(metrics)
	mb.metricVcenterVMVsanThroughput.emit(metrics)
}

func (mb *MetricsBuilder) EmitHost(metrics pdata.MetricSlice) {
	mb.metricVcenterHostCPUUsage.emit(metrics)
	mb.metricVcenterHostCPUUtilization.emit(metrics)
	mb.metricVcenterHostDiskLatencyAvg.emit(metrics)
	mb.metricVcenterHostDiskThroughput.emit(metrics)
	mb.metricVcenterHostMemoryUsage.emit(metrics)
	mb.metricVcenterHostMemoryUtilization.emit(metrics)
	mb.metricVcenterHostNetworkPackets.emit(metrics)
	mb.metricVcenterHostNetworkThroughput.emit(metrics)
	mb.metricVcenterHostVsanCacheHitRate.emit(metrics)
	mb.metricVcenterHostVsanCacheReads.emit(metrics)
	mb.metricVcenterHostVsanCongestions.emit(metrics)
	mb.metricVcenterHostVsanLatencyAvg.emit(metrics)
	mb.metricVcenterHostVsanOperations.emit(metrics)
	mb.metricVcenterHostVsanOutstandingIo.emit(metrics)
	mb.metricVcenterHostVsanThroughput.emit(metrics)
}

func (mb *MetricsBuilder) EmitResourcePool(metrics pdata.MetricSlice) {
	mb.metricVcenterResourcePoolCPUShares.emit(metrics)
	mb.metricVcenterResourcePoolCPUUsage.emit(metrics)
	mb.metricVcenterResourcePoolMemoryShares.emit(metrics)
	mb.metricVcenterResourcePoolMemoryUsage.emit(metrics)
}

func (mb *MetricsBuilder) EmitDatastore(metrics pdata.MetricSlice) {
	mb.metricVcenterDatastoreDiskUsage.emit(metrics)
	mb.metricVcenterDatastoreDiskUtilization.emit(metrics)
}

func (mb *MetricsBuilder) EmitDatacenter(metrics pdata.MetricSlice) {
	mb.metricVcenterDatacenterHostCount.emit(metrics)
	mb.metricVcenterDatacenterVMCount.emit(metrics)
}

func (mb *MetricsBuilder) EmitCluster(metrics pdata.MetricSlice) {
	mb.metricVcenterClusterCPUAvailable.emit(metrics)
	mb.metricVcenterClusterCPUEffective.emit(metrics)
	mb.metricVcenterClusterCPUUsed.emit(metrics)
	mb.metricVcenterClusterMemoryAvailable.emit(metrics)
	mb.metricVcenterClusterMemoryEffective.emit(metrics)
	mb.metricVcenterClusterMemoryUsed.emit(metrics)
	mb.metricVcenterClusterVsanCongestions.emit(metrics)
	mb.metricVcenterClusterVsanLatencyAvg.emit(metrics)
	mb.metricVcenterClusterVsanOperations.emit(metrics)
	mb.metricVcenterClusterVsanOutstandingIo.emit(metrics)
	mb.metricVcenterClusterVsanThroughput.emit(metrics)
}
