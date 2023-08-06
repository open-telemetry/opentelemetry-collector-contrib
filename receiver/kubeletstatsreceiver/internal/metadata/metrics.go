// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metadata // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/metadata"

import "go.opentelemetry.io/collector/pdata/pcommon"

type RecordDoubleDataPointFunc func(*ResourceMetricsBuilder, pcommon.Timestamp, float64)

type RecordIntDataPointFunc func(*ResourceMetricsBuilder, pcommon.Timestamp, int64)

type RecordIntDataPointWithDirectionFunc func(*ResourceMetricsBuilder, pcommon.Timestamp, int64, string, AttributeDirection)

type MetricsBuilders struct {
	NodeMetricsBuilder      *MetricsBuilder
	PodMetricsBuilder       *MetricsBuilder
	ContainerMetricsBuilder *MetricsBuilder
	OtherMetricsBuilder     *MetricsBuilder
}

type CPUMetrics struct {
	Time        RecordDoubleDataPointFunc
	Utilization RecordDoubleDataPointFunc
}

var NodeCPUMetrics = CPUMetrics{
	Time:        (*ResourceMetricsBuilder).RecordK8sNodeCPUTimeDataPoint,
	Utilization: (*ResourceMetricsBuilder).RecordK8sNodeCPUUtilizationDataPoint,
}

var PodCPUMetrics = CPUMetrics{
	Time:        (*ResourceMetricsBuilder).RecordK8sPodCPUTimeDataPoint,
	Utilization: (*ResourceMetricsBuilder).RecordK8sPodCPUUtilizationDataPoint,
}

var ContainerCPUMetrics = CPUMetrics{
	Time:        (*ResourceMetricsBuilder).RecordContainerCPUTimeDataPoint,
	Utilization: (*ResourceMetricsBuilder).RecordContainerCPUUtilizationDataPoint,
}

type MemoryMetrics struct {
	Available       RecordIntDataPointFunc
	Usage           RecordIntDataPointFunc
	Rss             RecordIntDataPointFunc
	WorkingSet      RecordIntDataPointFunc
	PageFaults      RecordIntDataPointFunc
	MajorPageFaults RecordIntDataPointFunc
}

var NodeMemoryMetrics = MemoryMetrics{
	Available:       (*ResourceMetricsBuilder).RecordK8sNodeMemoryAvailableDataPoint,
	Usage:           (*ResourceMetricsBuilder).RecordK8sNodeMemoryUsageDataPoint,
	Rss:             (*ResourceMetricsBuilder).RecordK8sNodeMemoryRssDataPoint,
	WorkingSet:      (*ResourceMetricsBuilder).RecordK8sNodeMemoryWorkingSetDataPoint,
	PageFaults:      (*ResourceMetricsBuilder).RecordK8sNodeMemoryPageFaultsDataPoint,
	MajorPageFaults: (*ResourceMetricsBuilder).RecordK8sNodeMemoryMajorPageFaultsDataPoint,
}

var PodMemoryMetrics = MemoryMetrics{
	Available:       (*ResourceMetricsBuilder).RecordK8sPodMemoryAvailableDataPoint,
	Usage:           (*ResourceMetricsBuilder).RecordK8sPodMemoryUsageDataPoint,
	Rss:             (*ResourceMetricsBuilder).RecordK8sPodMemoryRssDataPoint,
	WorkingSet:      (*ResourceMetricsBuilder).RecordK8sPodMemoryWorkingSetDataPoint,
	PageFaults:      (*ResourceMetricsBuilder).RecordK8sPodMemoryPageFaultsDataPoint,
	MajorPageFaults: (*ResourceMetricsBuilder).RecordK8sPodMemoryMajorPageFaultsDataPoint,
}

var ContainerMemoryMetrics = MemoryMetrics{
	Available:       (*ResourceMetricsBuilder).RecordContainerMemoryAvailableDataPoint,
	Usage:           (*ResourceMetricsBuilder).RecordContainerMemoryUsageDataPoint,
	Rss:             (*ResourceMetricsBuilder).RecordContainerMemoryRssDataPoint,
	WorkingSet:      (*ResourceMetricsBuilder).RecordContainerMemoryWorkingSetDataPoint,
	PageFaults:      (*ResourceMetricsBuilder).RecordContainerMemoryPageFaultsDataPoint,
	MajorPageFaults: (*ResourceMetricsBuilder).RecordContainerMemoryMajorPageFaultsDataPoint,
}

type FilesystemMetrics struct {
	Available RecordIntDataPointFunc
	Capacity  RecordIntDataPointFunc
	Usage     RecordIntDataPointFunc
}

var NodeFilesystemMetrics = FilesystemMetrics{
	Available: (*ResourceMetricsBuilder).RecordK8sNodeFilesystemAvailableDataPoint,
	Capacity:  (*ResourceMetricsBuilder).RecordK8sNodeFilesystemCapacityDataPoint,
	Usage:     (*ResourceMetricsBuilder).RecordK8sNodeFilesystemUsageDataPoint,
}

var PodFilesystemMetrics = FilesystemMetrics{
	Available: (*ResourceMetricsBuilder).RecordK8sPodFilesystemAvailableDataPoint,
	Capacity:  (*ResourceMetricsBuilder).RecordK8sPodFilesystemCapacityDataPoint,
	Usage:     (*ResourceMetricsBuilder).RecordK8sPodFilesystemUsageDataPoint,
}

var ContainerFilesystemMetrics = FilesystemMetrics{
	Available: (*ResourceMetricsBuilder).RecordContainerFilesystemAvailableDataPoint,
	Capacity:  (*ResourceMetricsBuilder).RecordContainerFilesystemCapacityDataPoint,
	Usage:     (*ResourceMetricsBuilder).RecordContainerFilesystemUsageDataPoint,
}

type NetworkMetrics struct {
	IO     RecordIntDataPointWithDirectionFunc
	Errors RecordIntDataPointWithDirectionFunc
}

var NodeNetworkMetrics = NetworkMetrics{
	IO:     (*ResourceMetricsBuilder).RecordK8sNodeNetworkIoDataPoint,
	Errors: (*ResourceMetricsBuilder).RecordK8sNodeNetworkErrorsDataPoint,
}

var PodNetworkMetrics = NetworkMetrics{
	IO:     (*ResourceMetricsBuilder).RecordK8sPodNetworkIoDataPoint,
	Errors: (*ResourceMetricsBuilder).RecordK8sPodNetworkErrorsDataPoint,
}

type VolumeMetrics struct {
	Available  RecordIntDataPointFunc
	Capacity   RecordIntDataPointFunc
	Inodes     RecordIntDataPointFunc
	InodesFree RecordIntDataPointFunc
	InodesUsed RecordIntDataPointFunc
}

var K8sVolumeMetrics = VolumeMetrics{
	Available:  (*ResourceMetricsBuilder).RecordK8sVolumeAvailableDataPoint,
	Capacity:   (*ResourceMetricsBuilder).RecordK8sVolumeCapacityDataPoint,
	Inodes:     (*ResourceMetricsBuilder).RecordK8sVolumeInodesDataPoint,
	InodesFree: (*ResourceMetricsBuilder).RecordK8sVolumeInodesFreeDataPoint,
	InodesUsed: (*ResourceMetricsBuilder).RecordK8sVolumeInodesUsedDataPoint,
}
