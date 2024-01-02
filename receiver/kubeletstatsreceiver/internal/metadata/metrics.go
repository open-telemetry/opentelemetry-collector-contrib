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
	Time               RecordDoubleDataPointFunc
	Utilization        RecordDoubleDataPointFunc
	LimitUtilization   RecordDoubleDataPointFunc
	RequestUtilization RecordDoubleDataPointFunc
}

var NodeCPUMetrics = CPUMetrics{
	Time:        (*MetricsBuilder).RecordK8sNodeCPUTimeDataPoint,
	Utilization: (*MetricsBuilder).RecordK8sNodeCPUUtilizationDataPoint,
}

var PodCPUMetrics = CPUMetrics{
	Time:               (*MetricsBuilder).RecordK8sPodCPUTimeDataPoint,
	Utilization:        (*MetricsBuilder).RecordK8sPodCPUUtilizationDataPoint,
	LimitUtilization:   (*MetricsBuilder).RecordK8sPodCPULimitUtilizationDataPoint,
	RequestUtilization: (*MetricsBuilder).RecordK8sPodCPURequestUtilizationDataPoint,
}

var ContainerCPUMetrics = CPUMetrics{
	Time:               (*MetricsBuilder).RecordContainerCPUTimeDataPoint,
	Utilization:        (*MetricsBuilder).RecordContainerCPUUtilizationDataPoint,
	LimitUtilization:   (*MetricsBuilder).RecordK8sContainerCPULimitUtilizationDataPoint,
	RequestUtilization: (*MetricsBuilder).RecordK8sContainerCPURequestUtilizationDataPoint,
}

type MemoryMetrics struct {
	Available          RecordIntDataPointFunc
	Usage              RecordIntDataPointFunc
	LimitUtilization   RecordDoubleDataPointFunc
	RequestUtilization RecordDoubleDataPointFunc
	Rss                RecordIntDataPointFunc
	WorkingSet         RecordIntDataPointFunc
	PageFaults         RecordIntDataPointFunc
	MajorPageFaults    RecordIntDataPointFunc
}

var NodeMemoryMetrics = MemoryMetrics{
	Available:       (*MetricsBuilder).RecordK8sNodeMemoryAvailableDataPoint,
	Usage:           (*MetricsBuilder).RecordK8sNodeMemoryUsageDataPoint,
	Rss:             (*MetricsBuilder).RecordK8sNodeMemoryRssDataPoint,
	WorkingSet:      (*MetricsBuilder).RecordK8sNodeMemoryWorkingSetDataPoint,
	PageFaults:      (*MetricsBuilder).RecordK8sNodeMemoryPageFaultsDataPoint,
	MajorPageFaults: (*MetricsBuilder).RecordK8sNodeMemoryMajorPageFaultsDataPoint,
}

var PodMemoryMetrics = MemoryMetrics{
	Available:          (*MetricsBuilder).RecordK8sPodMemoryAvailableDataPoint,
	Usage:              (*MetricsBuilder).RecordK8sPodMemoryUsageDataPoint,
	LimitUtilization:   (*MetricsBuilder).RecordK8sPodMemoryLimitUtilizationDataPoint,
	RequestUtilization: (*MetricsBuilder).RecordK8sPodMemoryRequestUtilizationDataPoint,
	Rss:                (*MetricsBuilder).RecordK8sPodMemoryRssDataPoint,
	WorkingSet:         (*MetricsBuilder).RecordK8sPodMemoryWorkingSetDataPoint,
	PageFaults:         (*MetricsBuilder).RecordK8sPodMemoryPageFaultsDataPoint,
	MajorPageFaults:    (*MetricsBuilder).RecordK8sPodMemoryMajorPageFaultsDataPoint,
}

var ContainerMemoryMetrics = MemoryMetrics{
	Available:          (*MetricsBuilder).RecordContainerMemoryAvailableDataPoint,
	Usage:              (*MetricsBuilder).RecordContainerMemoryUsageDataPoint,
	LimitUtilization:   (*MetricsBuilder).RecordK8sContainerMemoryLimitUtilizationDataPoint,
	RequestUtilization: (*MetricsBuilder).RecordK8sContainerMemoryRequestUtilizationDataPoint,
	Rss:                (*MetricsBuilder).RecordContainerMemoryRssDataPoint,
	WorkingSet:         (*MetricsBuilder).RecordContainerMemoryWorkingSetDataPoint,
	PageFaults:         (*MetricsBuilder).RecordContainerMemoryPageFaultsDataPoint,
	MajorPageFaults:    (*MetricsBuilder).RecordContainerMemoryMajorPageFaultsDataPoint,
}

type FilesystemMetrics struct {
	Available RecordIntDataPointFunc
	Capacity  RecordIntDataPointFunc
	Usage     RecordIntDataPointFunc
}

var NodeFilesystemMetrics = FilesystemMetrics{
	Available: (*MetricsBuilder).RecordK8sNodeFilesystemAvailableDataPoint,
	Capacity:  (*MetricsBuilder).RecordK8sNodeFilesystemCapacityDataPoint,
	Usage:     (*MetricsBuilder).RecordK8sNodeFilesystemUsageDataPoint,
}

var PodFilesystemMetrics = FilesystemMetrics{
	Available: (*MetricsBuilder).RecordK8sPodFilesystemAvailableDataPoint,
	Capacity:  (*MetricsBuilder).RecordK8sPodFilesystemCapacityDataPoint,
	Usage:     (*MetricsBuilder).RecordK8sPodFilesystemUsageDataPoint,
}

var ContainerFilesystemMetrics = FilesystemMetrics{
	Available: (*MetricsBuilder).RecordContainerFilesystemAvailableDataPoint,
	Capacity:  (*MetricsBuilder).RecordContainerFilesystemCapacityDataPoint,
	Usage:     (*MetricsBuilder).RecordContainerFilesystemUsageDataPoint,
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

type UptimeMetrics struct {
	Uptime RecordIntDataPointFunc
}

var NodeUptimeMetrics = UptimeMetrics{
	Uptime: (*MetricsBuilder).RecordK8sNodeUptimeDataPoint,
}

var PodUptimeMetrics = UptimeMetrics{
	Uptime: (*MetricsBuilder).RecordK8sPodUptimeDataPoint,
}

var ContainerUptimeMetrics = UptimeMetrics{
	Uptime: (*MetricsBuilder).RecordContainerUptimeDataPoint,
}
