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

package metadata

type CpuMetrics struct {
	Time        MetricIntf
	Utilization MetricIntf
}

var NodeCpuMetrics = CpuMetrics{
	Time:        M.K8sNodeCPUTime,
	Utilization: M.K8sNodeCPUUtilization,
}

var PodCpuMetrics = CpuMetrics{
	Time:        M.K8sPodCPUTime,
	Utilization: M.K8sPodCPUUtilization,
}

var ContainerCpuMetrics = CpuMetrics{
	Time:        M.ContainerCPUTime,
	Utilization: M.ContainerCPUUtilization,
}

type MemoryMetrics struct {
	Available       MetricIntf
	Usage           MetricIntf
	Rss             MetricIntf
	WorkingSet      MetricIntf
	PageFaults      MetricIntf
	MajorPageFaults MetricIntf
}

var NodeMemoryMetrics = MemoryMetrics{
	Available:       M.K8sNodeMemoryAvailable,
	Usage:           M.K8sNodeMemoryUsage,
	Rss:             M.K8sNodeMemoryRss,
	WorkingSet:      M.K8sNodeMemoryWorkingSet,
	PageFaults:      M.K8sNodeMemoryPageFaults,
	MajorPageFaults: M.K8sNodeMemoryMajorPageFaults,
}

var PodMemoryMetrics = MemoryMetrics{
	Available:       M.K8sPodMemoryAvailable,
	Usage:           M.K8sPodMemoryUsage,
	Rss:             M.K8sPodMemoryRss,
	WorkingSet:      M.K8sPodMemoryWorkingSet,
	PageFaults:      M.K8sPodMemoryPageFaults,
	MajorPageFaults: M.K8sPodMemoryMajorPageFaults,
}

var ContainerMemoryMetrics = MemoryMetrics{
	Available:       M.ContainerMemoryAvailable,
	Usage:           M.ContainerMemoryUsage,
	Rss:             M.ContainerMemoryRss,
	WorkingSet:      M.ContainerMemoryWorkingSet,
	PageFaults:      M.ContainerMemoryPageFaults,
	MajorPageFaults: M.ContainerMemoryMajorPageFaults,
}

type FilesystemMetrics struct {
	Available MetricIntf
	Capacity  MetricIntf
	Usage     MetricIntf
}

var NodeFilesystemMetrics = FilesystemMetrics{
	Available: M.K8sNodeFilesystemAvailable,
	Capacity:  M.K8sNodeFilesystemCapacity,
	Usage:     M.K8sNodeFilesystemUsage,
}

var PodFilesystemMetrics = FilesystemMetrics{
	Available: M.K8sPodFilesystemAvailable,
	Capacity:  M.K8sPodFilesystemCapacity,
	Usage:     M.K8sPodFilesystemUsage,
}

var ContainerFilesystemMetrics = FilesystemMetrics{
	Available: M.ContainerFilesystemAvailable,
	Capacity:  M.ContainerFilesystemCapacity,
	Usage:     M.ContainerFilesystemUsage,
}

type NetworkMetrics struct {
	IO     MetricIntf
	Errors MetricIntf
}

var NodeNetworkMetrics = NetworkMetrics{
	IO:     M.K8sNodeNetworkIo,
	Errors: M.K8sNodeNetworkErrors,
}

var PodNetworkMetrics = NetworkMetrics{
	IO:     M.K8sPodNetworkIo,
	Errors: M.K8sPodNetworkErrors,
}

type VolumeMetrics struct {
	Available  MetricIntf
	Capacity   MetricIntf
	Inodes     MetricIntf
	InodesFree MetricIntf
	InodesUsed MetricIntf
}

var K8sVolumeMetrics = VolumeMetrics{
	Available:  M.K8sVolumeAvailable,
	Capacity:   M.K8sVolumeCapacity,
	Inodes:     M.K8sVolumeInodes,
	InodesFree: M.K8sVolumeInodesFree,
	InodesUsed: M.K8sVolumeInodesUsed,
}
