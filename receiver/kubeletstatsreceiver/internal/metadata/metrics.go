// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metadata // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/metadata"

import "go.opentelemetry.io/collector/pdata/pcommon"

type RecordDoubleDataPointFunc func(*MetricsBuilder, pcommon.Timestamp, float64)

type RecordIntDataPointFunc func(*MetricsBuilder, pcommon.Timestamp, int64)

type RecordIntDataPointWithDirectionFunc func(*MetricsBuilder, pcommon.Timestamp, int64, string, AttributeDirection)

type MetricsBuilders struct {
	NodeMetricsBuilder      *MetricsBuilder
	PodMetricsBuilder       *MetricsBuilder
	ContainerMetricsBuilder *MetricsBuilder
	OtherMetricsBuilder     *MetricsBuilder
}

type CPUMetrics struct {
	Time        []RecordDoubleDataPointFunc
	Utilization []RecordDoubleDataPointFunc
}

var NodeCPUMetrics = CPUMetrics{
	Time:        []RecordDoubleDataPointFunc{(*MetricsBuilder).RecordK8sNodeCPUTimeDataPoint},
	Utilization: []RecordDoubleDataPointFunc{(*MetricsBuilder).RecordK8sNodeCPUUtilizationDataPoint},
}

var PodCPUMetrics = CPUMetrics{
	Time:        []RecordDoubleDataPointFunc{(*MetricsBuilder).RecordK8sPodCPUTimeDataPoint},
	Utilization: []RecordDoubleDataPointFunc{(*MetricsBuilder).RecordK8sPodCPUUtilizationDataPoint},
}

var ContainerCPUMetrics = CPUMetrics{
	Time:        []RecordDoubleDataPointFunc{(*MetricsBuilder).RecordContainerCPUTimeDataPoint, (*MetricsBuilder).RecordK8sContainerCPUTimeDataPoint},
	Utilization: []RecordDoubleDataPointFunc{(*MetricsBuilder).RecordContainerCPUUtilizationDataPoint, (*MetricsBuilder).RecordK8sContainerCPUUtilizationDataPoint},
}

type MemoryMetrics struct {
	Available       []RecordIntDataPointFunc
	Usage           []RecordIntDataPointFunc
	Rss             []RecordIntDataPointFunc
	WorkingSet      []RecordIntDataPointFunc
	PageFaults      []RecordIntDataPointFunc
	MajorPageFaults []RecordIntDataPointFunc
}

var NodeMemoryMetrics = MemoryMetrics{
	Available:       []RecordIntDataPointFunc{(*MetricsBuilder).RecordK8sNodeMemoryAvailableDataPoint},
	Usage:           []RecordIntDataPointFunc{(*MetricsBuilder).RecordK8sNodeMemoryUsageDataPoint},
	Rss:             []RecordIntDataPointFunc{(*MetricsBuilder).RecordK8sNodeMemoryRssDataPoint},
	WorkingSet:      []RecordIntDataPointFunc{(*MetricsBuilder).RecordK8sNodeMemoryWorkingSetDataPoint},
	PageFaults:      []RecordIntDataPointFunc{(*MetricsBuilder).RecordK8sNodeMemoryPageFaultsDataPoint},
	MajorPageFaults: []RecordIntDataPointFunc{(*MetricsBuilder).RecordK8sNodeMemoryMajorPageFaultsDataPoint},
}

var PodMemoryMetrics = MemoryMetrics{
	Available:       []RecordIntDataPointFunc{(*MetricsBuilder).RecordK8sPodMemoryAvailableDataPoint},
	Usage:           []RecordIntDataPointFunc{(*MetricsBuilder).RecordK8sPodMemoryUsageDataPoint},
	Rss:             []RecordIntDataPointFunc{(*MetricsBuilder).RecordK8sPodMemoryRssDataPoint},
	WorkingSet:      []RecordIntDataPointFunc{(*MetricsBuilder).RecordK8sPodMemoryWorkingSetDataPoint},
	PageFaults:      []RecordIntDataPointFunc{(*MetricsBuilder).RecordK8sPodMemoryPageFaultsDataPoint},
	MajorPageFaults: []RecordIntDataPointFunc{(*MetricsBuilder).RecordK8sPodMemoryMajorPageFaultsDataPoint},
}

var ContainerMemoryMetrics = MemoryMetrics{
	Available:       []RecordIntDataPointFunc{(*MetricsBuilder).RecordContainerMemoryAvailableDataPoint, (*MetricsBuilder).RecordK8sContainerMemoryAvailableDataPoint},
	Usage:           []RecordIntDataPointFunc{(*MetricsBuilder).RecordContainerMemoryUsageDataPoint, (*MetricsBuilder).RecordK8sContainerMemoryUsageDataPoint},
	Rss:             []RecordIntDataPointFunc{(*MetricsBuilder).RecordContainerMemoryRssDataPoint, (*MetricsBuilder).RecordK8sContainerMemoryRssDataPoint},
	WorkingSet:      []RecordIntDataPointFunc{(*MetricsBuilder).RecordContainerMemoryWorkingSetDataPoint, (*MetricsBuilder).RecordK8sContainerMemoryWorkingSetDataPoint},
	PageFaults:      []RecordIntDataPointFunc{(*MetricsBuilder).RecordContainerMemoryPageFaultsDataPoint, (*MetricsBuilder).RecordK8sContainerMemoryPageFaultsDataPoint},
	MajorPageFaults: []RecordIntDataPointFunc{(*MetricsBuilder).RecordContainerMemoryMajorPageFaultsDataPoint, (*MetricsBuilder).RecordK8sContainerMemoryMajorPageFaultsDataPoint},
}

type FilesystemMetrics struct {
	Available []RecordIntDataPointFunc
	Capacity  []RecordIntDataPointFunc
	Usage     []RecordIntDataPointFunc
}

var NodeFilesystemMetrics = FilesystemMetrics{
	Available: []RecordIntDataPointFunc{(*MetricsBuilder).RecordK8sNodeFilesystemAvailableDataPoint},
	Capacity:  []RecordIntDataPointFunc{(*MetricsBuilder).RecordK8sNodeFilesystemCapacityDataPoint},
	Usage:     []RecordIntDataPointFunc{(*MetricsBuilder).RecordK8sNodeFilesystemUsageDataPoint},
}

var PodFilesystemMetrics = FilesystemMetrics{
	Available: []RecordIntDataPointFunc{(*MetricsBuilder).RecordK8sPodFilesystemAvailableDataPoint},
	Capacity:  []RecordIntDataPointFunc{(*MetricsBuilder).RecordK8sPodFilesystemCapacityDataPoint},
	Usage:     []RecordIntDataPointFunc{(*MetricsBuilder).RecordK8sPodFilesystemUsageDataPoint},
}

var ContainerFilesystemMetrics = FilesystemMetrics{
	Available: []RecordIntDataPointFunc{(*MetricsBuilder).RecordContainerFilesystemAvailableDataPoint, (*MetricsBuilder).RecordK8sContainerFilesystemAvailableDataPoint},
	Capacity:  []RecordIntDataPointFunc{(*MetricsBuilder).RecordContainerFilesystemCapacityDataPoint, (*MetricsBuilder).RecordK8sContainerFilesystemCapacityDataPoint},
	Usage:     []RecordIntDataPointFunc{(*MetricsBuilder).RecordContainerFilesystemUsageDataPoint, (*MetricsBuilder).RecordK8sContainerFilesystemUsageDataPoint},
}

type NetworkMetrics struct {
	IO     RecordIntDataPointWithDirectionFunc
	Errors RecordIntDataPointWithDirectionFunc
}

var NodeNetworkMetrics = NetworkMetrics{
	IO:     (*MetricsBuilder).RecordK8sNodeNetworkIoDataPoint,
	Errors: (*MetricsBuilder).RecordK8sNodeNetworkErrorsDataPoint,
}

var PodNetworkMetrics = NetworkMetrics{
	IO:     (*MetricsBuilder).RecordK8sPodNetworkIoDataPoint,
	Errors: (*MetricsBuilder).RecordK8sPodNetworkErrorsDataPoint,
}

type VolumeMetrics struct {
	Available  RecordIntDataPointFunc
	Capacity   RecordIntDataPointFunc
	Inodes     RecordIntDataPointFunc
	InodesFree RecordIntDataPointFunc
	InodesUsed RecordIntDataPointFunc
}

var K8sVolumeMetrics = VolumeMetrics{
	Available:  (*MetricsBuilder).RecordK8sVolumeAvailableDataPoint,
	Capacity:   (*MetricsBuilder).RecordK8sVolumeCapacityDataPoint,
	Inodes:     (*MetricsBuilder).RecordK8sVolumeInodesDataPoint,
	InodesFree: (*MetricsBuilder).RecordK8sVolumeInodesFreeDataPoint,
	InodesUsed: (*MetricsBuilder).RecordK8sVolumeInodesUsedDataPoint,
}
