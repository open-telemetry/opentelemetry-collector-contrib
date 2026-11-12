// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metadata // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/metadata"

import "go.opentelemetry.io/collector/pdata/pcommon"

type RecordDoubleDataPointFunc func(*MetricsBuilder, pcommon.Timestamp, float64)

type RecordIntDataPointFunc func(*MetricsBuilder, pcommon.Timestamp, int64)

type RecordIntDataPointWithDirectionFunc func(*MetricsBuilder, pcommon.Timestamp, int64, string, AttributeDirection)

type RecordIntDataPointWithFsTypeFunc func(*MetricsBuilder, pcommon.Timestamp, int64, AttributeFsType)

type MetricsBuilders struct {
	NodeMetricsBuilder      *MetricsBuilder
	PodMetricsBuilder       *MetricsBuilder
	ContainerMetricsBuilder *MetricsBuilder
	OtherMetricsBuilder     *MetricsBuilder
}

type CPUMetrics struct {
	Time               RecordDoubleDataPointFunc
	Usage              RecordDoubleDataPointFunc
	NodeUtilization    RecordDoubleDataPointFunc
	LimitUtilization   RecordDoubleDataPointFunc
	RequestUtilization RecordDoubleDataPointFunc
}

var NodeCPUMetrics = CPUMetrics{
	Time:  (*MetricsBuilder).RecordK8sNodeCPUTimeDataPoint,
	Usage: (*MetricsBuilder).RecordK8sNodeCPUUsageDataPoint,
}

var PodCPUMetrics = CPUMetrics{
	Time:               (*MetricsBuilder).RecordK8sPodCPUTimeDataPoint,
	Usage:              (*MetricsBuilder).RecordK8sPodCPUUsageDataPoint,
	NodeUtilization:    (*MetricsBuilder).RecordK8sPodCPUNodeUtilizationDataPoint,
	LimitUtilization:   (*MetricsBuilder).RecordK8sPodCPULimitUtilizationDataPoint,
	RequestUtilization: (*MetricsBuilder).RecordK8sPodCPURequestUtilizationDataPoint,
}

var ContainerCPUMetrics = CPUMetrics{
	Time:               (*MetricsBuilder).RecordContainerCPUTimeDataPoint,
	Usage:              (*MetricsBuilder).RecordContainerCPUUsageDataPoint,
	NodeUtilization:    (*MetricsBuilder).RecordK8sContainerCPUNodeUtilizationDataPoint,
	LimitUtilization:   (*MetricsBuilder).RecordK8sContainerCPULimitUtilizationDataPoint,
	RequestUtilization: (*MetricsBuilder).RecordK8sContainerCPURequestUtilizationDataPoint,
}

var SystemContainerCPUMetrics = CPUMetrics{
	Time:  (*MetricsBuilder).RecordK8sNodeSystemContainerCPUTimeDataPoint,
	Usage: (*MetricsBuilder).RecordK8sNodeSystemContainerCPUUsageDataPoint,
}

type MemoryMetrics struct {
	Available          RecordIntDataPointFunc
	Usage              RecordIntDataPointFunc
	NodeUtilization    RecordDoubleDataPointFunc
	LimitUtilization   RecordDoubleDataPointFunc
	RequestUtilization RecordDoubleDataPointFunc
	// LimitUtilizationV1 and RequestUtilizationV1 record the semantic
	// conventions compliant k8s.{pod,container}.memory.{limit,request}.utilization
	// metrics, which use the memory working set as the numerator.
	LimitUtilizationV1   RecordDoubleDataPointFunc
	RequestUtilizationV1 RecordDoubleDataPointFunc
	Rss                  RecordIntDataPointFunc
	WorkingSet           RecordIntDataPointFunc
	PageFaults           RecordIntDataPointFunc
	MajorPageFaults      RecordIntDataPointFunc
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
	Available:            (*MetricsBuilder).RecordK8sPodMemoryAvailableDataPoint,
	Usage:                (*MetricsBuilder).RecordK8sPodMemoryUsageDataPoint,
	NodeUtilization:      (*MetricsBuilder).RecordK8sPodMemoryNodeUtilizationDataPoint,
	LimitUtilization:     (*MetricsBuilder).RecordK8sPodMemoryLimitUtilizationDataPoint,
	RequestUtilization:   (*MetricsBuilder).RecordK8sPodMemoryRequestUtilizationDataPoint,
	LimitUtilizationV1:   (*MetricsBuilder).recordK8sPodMemoryLimitUtilizationV1DataPoint,
	RequestUtilizationV1: (*MetricsBuilder).recordK8sPodMemoryRequestUtilizationV1DataPoint,
	Rss:                  (*MetricsBuilder).RecordK8sPodMemoryRssDataPoint,
	WorkingSet:           (*MetricsBuilder).RecordK8sPodMemoryWorkingSetDataPoint,
	PageFaults:           (*MetricsBuilder).RecordK8sPodMemoryPageFaultsDataPoint,
	MajorPageFaults:      (*MetricsBuilder).RecordK8sPodMemoryMajorPageFaultsDataPoint,
}

var ContainerMemoryMetrics = MemoryMetrics{
	Available:            (*MetricsBuilder).RecordContainerMemoryAvailableDataPoint,
	Usage:                (*MetricsBuilder).RecordContainerMemoryUsageDataPoint,
	NodeUtilization:      (*MetricsBuilder).RecordK8sContainerMemoryNodeUtilizationDataPoint,
	LimitUtilization:     (*MetricsBuilder).RecordK8sContainerMemoryLimitUtilizationDataPoint,
	RequestUtilization:   (*MetricsBuilder).RecordK8sContainerMemoryRequestUtilizationDataPoint,
	LimitUtilizationV1:   (*MetricsBuilder).recordK8sContainerMemoryLimitUtilizationV1DataPoint,
	RequestUtilizationV1: (*MetricsBuilder).recordK8sContainerMemoryRequestUtilizationV1DataPoint,
	Rss:                  (*MetricsBuilder).RecordContainerMemoryRssDataPoint,
	WorkingSet:           (*MetricsBuilder).RecordContainerMemoryWorkingSetDataPoint,
	PageFaults:           (*MetricsBuilder).RecordContainerMemoryPageFaultsDataPoint,
	MajorPageFaults:      (*MetricsBuilder).RecordContainerMemoryMajorPageFaultsDataPoint,
}

// The memory {limit,request} utilization metrics cannot use the mdatagen
// metric migration, because the generated dual-emission code records the same
// value for the legacy and the target metric. The legacy metrics use the memory
// usage as the numerator, while the semantic conventions use the memory working
// set. mdatagen does not generate Record functions for versioned (@v1) metrics,
// so the recorders below are provided here instead.

func (mb *MetricsBuilder) recordK8sPodMemoryLimitUtilizationV1DataPoint(ts pcommon.Timestamp, val float64) {
	if ReceiverKubeletStatsEmitV1K8sConventionsFeatureGate.IsEnabled() {
		mb.metricK8sPodMemoryLimitUtilizationV1.recordDataPoint(mb.startTime, ts, val)
	}
}

func (mb *MetricsBuilder) recordK8sPodMemoryRequestUtilizationV1DataPoint(ts pcommon.Timestamp, val float64) {
	if ReceiverKubeletStatsEmitV1K8sConventionsFeatureGate.IsEnabled() {
		mb.metricK8sPodMemoryRequestUtilizationV1.recordDataPoint(mb.startTime, ts, val)
	}
}

func (mb *MetricsBuilder) recordK8sContainerMemoryLimitUtilizationV1DataPoint(ts pcommon.Timestamp, val float64) {
	if ReceiverKubeletStatsEmitV1K8sConventionsFeatureGate.IsEnabled() {
		mb.metricK8sContainerMemoryLimitUtilizationV1.recordDataPoint(mb.startTime, ts, val)
	}
}

func (mb *MetricsBuilder) recordK8sContainerMemoryRequestUtilizationV1DataPoint(ts pcommon.Timestamp, val float64) {
	if ReceiverKubeletStatsEmitV1K8sConventionsFeatureGate.IsEnabled() {
		mb.metricK8sContainerMemoryRequestUtilizationV1.recordDataPoint(mb.startTime, ts, val)
	}
}

var SystemContainerMemoryMetrics = MemoryMetrics{
	Usage:      (*MetricsBuilder).RecordK8sNodeSystemContainerMemoryUsageDataPoint,
	WorkingSet: (*MetricsBuilder).RecordK8sNodeSystemContainerMemoryWorkingSetDataPoint,
}

type FilesystemMetrics struct {
	Available  RecordIntDataPointFunc
	Capacity   RecordIntDataPointFunc
	Usage      RecordIntDataPointFunc
	Inodes     RecordIntDataPointFunc
	InodesFree RecordIntDataPointFunc
}

type EphemeralStorageMetrics struct {
	Usage RecordIntDataPointWithFsTypeFunc
}

var NodeFilesystemMetrics = FilesystemMetrics{
	Available:  (*MetricsBuilder).RecordK8sNodeFilesystemAvailableDataPoint,
	Capacity:   (*MetricsBuilder).RecordK8sNodeFilesystemCapacityDataPoint,
	Usage:      (*MetricsBuilder).RecordK8sNodeFilesystemUsageDataPoint,
	Inodes:     (*MetricsBuilder).RecordK8sNodeFilesystemInodeCountDataPoint,
	InodesFree: (*MetricsBuilder).RecordK8sNodeFilesystemInodeFreeDataPoint,
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

var ContainerEphemeralStorageMetrics = EphemeralStorageMetrics{
	Usage: (*MetricsBuilder).RecordK8sContainerEphemeralStorageUsageDataPoint,
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
	Used       RecordIntDataPointFunc
	Inodes     RecordIntDataPointFunc
	InodesFree RecordIntDataPointFunc
	InodesUsed RecordIntDataPointFunc
}

var K8sVolumeMetrics = VolumeMetrics{
	Available:  (*MetricsBuilder).RecordK8sVolumeAvailableDataPoint,
	Capacity:   (*MetricsBuilder).RecordK8sVolumeCapacityDataPoint,
	Used:       (*MetricsBuilder).RecordK8sPodVolumeUsageDataPoint,
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
