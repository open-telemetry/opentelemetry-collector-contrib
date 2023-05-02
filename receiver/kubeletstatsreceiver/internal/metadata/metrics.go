// Copyright The OpenTelemetry Authors
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
	Time        RecordDoubleDataPointFunc
	Utilization RecordDoubleDataPointFunc
}

var NodeCPUMetrics = CPUMetrics{
	Time:        (*MetricsBuilder).RecordK8sNodeCPUTimeDataPoint,
	Utilization: (*MetricsBuilder).RecordK8sNodeCPUUtilizationDataPoint,
}

var PodCPUMetrics = CPUMetrics{
	Time:        (*MetricsBuilder).RecordK8sPodCPUTimeDataPoint,
	Utilization: (*MetricsBuilder).RecordK8sPodCPUUtilizationDataPoint,
}

var ContainerCPUMetrics = CPUMetrics{
	Time:        (*MetricsBuilder).RecordContainerCPUTimeDataPoint,
	Utilization: (*MetricsBuilder).RecordContainerCPUUtilizationDataPoint,
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
	Available:       (*MetricsBuilder).RecordK8sNodeMemoryAvailableDataPoint,
	Usage:           (*MetricsBuilder).RecordK8sNodeMemoryUsageDataPoint,
	Rss:             (*MetricsBuilder).RecordK8sNodeMemoryRssDataPoint,
	WorkingSet:      (*MetricsBuilder).RecordK8sNodeMemoryWorkingSetDataPoint,
	PageFaults:      (*MetricsBuilder).RecordK8sNodeMemoryPageFaultsDataPoint,
	MajorPageFaults: (*MetricsBuilder).RecordK8sNodeMemoryMajorPageFaultsDataPoint,
}

var PodMemoryMetrics = MemoryMetrics{
	Available:       (*MetricsBuilder).RecordK8sPodMemoryAvailableDataPoint,
	Usage:           (*MetricsBuilder).RecordK8sPodMemoryUsageDataPoint,
	Rss:             (*MetricsBuilder).RecordK8sPodMemoryRssDataPoint,
	WorkingSet:      (*MetricsBuilder).RecordK8sPodMemoryWorkingSetDataPoint,
	PageFaults:      (*MetricsBuilder).RecordK8sPodMemoryPageFaultsDataPoint,
	MajorPageFaults: (*MetricsBuilder).RecordK8sPodMemoryMajorPageFaultsDataPoint,
}

var ContainerMemoryMetrics = MemoryMetrics{
	Available:       (*MetricsBuilder).RecordContainerMemoryAvailableDataPoint,
	Usage:           (*MetricsBuilder).RecordContainerMemoryUsageDataPoint,
	Rss:             (*MetricsBuilder).RecordContainerMemoryRssDataPoint,
	WorkingSet:      (*MetricsBuilder).RecordContainerMemoryWorkingSetDataPoint,
	PageFaults:      (*MetricsBuilder).RecordContainerMemoryPageFaultsDataPoint,
	MajorPageFaults: (*MetricsBuilder).RecordContainerMemoryMajorPageFaultsDataPoint,
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
