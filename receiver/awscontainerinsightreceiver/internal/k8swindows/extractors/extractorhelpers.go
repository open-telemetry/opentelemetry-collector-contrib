package extractors

import (
	"fmt"

	stats "k8s.io/kubelet/pkg/apis/stats/v1alpha1"
)

// convertCPUStats Convert kubelet CPU stats to Raw CPU stats
func convertCPUStats(kubeletCPUStat stats.CPUStats) CPUStat {
	var cpuStat CPUStat

	cpuStat.Time = kubeletCPUStat.Time.Time

	if kubeletCPUStat.UsageCoreNanoSeconds != nil {
		cpuStat.UsageCoreNanoSeconds = *kubeletCPUStat.UsageCoreNanoSeconds
	}
	if kubeletCPUStat.UsageNanoCores != nil {
		cpuStat.UsageNanoCores = *kubeletCPUStat.UsageNanoCores
	}

	return cpuStat
}

// convertFileSystemStats Convert kubelet file system stats to Raw memory stats
func convertFileSystemStats(kubeletFSstat stats.FsStats) FileSystemStat {
	var fsstat FileSystemStat

	fsstat.Time = kubeletFSstat.Time.Time

	if kubeletFSstat.UsedBytes != nil {
		fsstat.UsedBytes = *kubeletFSstat.UsedBytes
	}

	if kubeletFSstat.AvailableBytes != nil {
		fsstat.AvailableBytes = *kubeletFSstat.AvailableBytes
	}

	if kubeletFSstat.CapacityBytes != nil {
		fsstat.CapacityBytes = *kubeletFSstat.CapacityBytes
	}

	return fsstat
}

// convertMemoryStats Convert kubelet memory stats to Raw memory stats
func convertMemoryStats(kubeletMemoryStat stats.MemoryStats) MemoryStat {
	var memoryStat MemoryStat

	memoryStat.Time = kubeletMemoryStat.Time.Time

	if kubeletMemoryStat.UsageBytes != nil {
		memoryStat.UsageBytes = *kubeletMemoryStat.UsageBytes
	}
	if kubeletMemoryStat.AvailableBytes != nil {
		memoryStat.AvailableBytes = *kubeletMemoryStat.AvailableBytes
	}
	if kubeletMemoryStat.WorkingSetBytes != nil {
		memoryStat.WorkingSetBytes = *kubeletMemoryStat.WorkingSetBytes
	}
	if kubeletMemoryStat.RSSBytes != nil {
		memoryStat.RSSBytes = *kubeletMemoryStat.RSSBytes
	}
	if kubeletMemoryStat.PageFaults != nil {
		memoryStat.PageFaults = *kubeletMemoryStat.PageFaults
	}
	if kubeletMemoryStat.MajorPageFaults != nil {
		memoryStat.MajorPageFaults = *kubeletMemoryStat.MajorPageFaults
	}
	return memoryStat
}

// convertNetworkStats Convert kubelet network system stats to Raw memory stats
func convertNetworkStats(kubeletNetworkStat stats.NetworkStats, kubeletIntfStat stats.InterfaceStats) NetworkStat {
	var networkstat NetworkStat

	networkstat.Time = kubeletNetworkStat.Time.Time

	networkstat.Name = kubeletIntfStat.Name

	if kubeletIntfStat.TxBytes != nil {
		networkstat.TxBytes = *kubeletIntfStat.TxBytes
	}

	if kubeletIntfStat.TxErrors != nil {
		networkstat.TxErrors = *kubeletIntfStat.TxErrors
	}

	if kubeletIntfStat.RxBytes != nil {
		networkstat.RxBytes = *kubeletIntfStat.RxBytes
	}

	if kubeletIntfStat.RxErrors != nil {
		networkstat.RxErrors = *kubeletIntfStat.RxErrors
	}

	return networkstat
}

// convertHCSNetworkStats Convert HCS network system stats to Raw network stats
func convertHCSNetworkStats(stat HCSStat, networkStat HCSNetworkStat) NetworkStat {
	var networkstat NetworkStat

	networkstat.Time = stat.Time

	networkstat.Name = networkStat.Name
	networkstat.TxBytes = networkStat.BytesSent
	networkstat.RxBytes = networkStat.BytesReceived
	networkstat.DroppedIncoming = networkStat.DroppedPacketsIncoming
	networkstat.DroppedOutgoing = networkStat.DroppedPacketsOutgoing

	return networkstat
}

// ConvertPodToRaw Converts Kubelet Pod stats to RawMetric.
func ConvertPodToRaw(podStat stats.PodStats) RawMetric {
	var rawMetic RawMetric

	rawMetic.Id = podStat.PodRef.UID
	rawMetic.Name = podStat.PodRef.Name
	rawMetic.Namespace = podStat.PodRef.Namespace

	if podStat.CPU != nil {
		rawMetic.Time = podStat.CPU.Time.Time
		rawMetic.CPUStats = convertCPUStats(*podStat.CPU)
	}

	if podStat.Memory != nil {
		if rawMetic.Time.IsZero() {
			rawMetic.Time = podStat.Memory.Time.Time
		}
		rawMetic.MemoryStats = convertMemoryStats(*podStat.Memory)
	}

	if podStat.Network != nil {
		for _, intfStats := range podStat.Network.Interfaces {
			rawMetic.NetworkStats = append(rawMetic.NetworkStats, convertNetworkStats(*podStat.Network, intfStats))
		}
	}

	return rawMetic
}

// ConvertContainerToRaw Converts Kubelet Container stats per Pod to RawMetric.
func ConvertContainerToRaw(containerStat stats.ContainerStats, podStat stats.PodStats) RawMetric {
	var rawMetic RawMetric

	rawMetic.Id = fmt.Sprintf("%s-%s", podStat.PodRef.UID, containerStat.Name)
	rawMetic.Name = containerStat.Name
	rawMetic.Namespace = podStat.PodRef.Namespace

	if containerStat.CPU != nil {
		rawMetic.Time = containerStat.CPU.Time.Time
		rawMetic.CPUStats = convertCPUStats(*containerStat.CPU)
	}

	if containerStat.Memory != nil {
		if rawMetic.Time.IsZero() {
			rawMetic.Time = containerStat.Memory.Time.Time
		}
		rawMetic.MemoryStats = convertMemoryStats(*containerStat.Memory)
	}

	rawMetic.FileSystemStats = []FileSystemStat{}
	if containerStat.Rootfs != nil {
		fStat := convertFileSystemStats(*containerStat.Rootfs)
		fStat.Name = "rootfs"
		rawMetic.FileSystemStats = append(rawMetic.FileSystemStats, fStat)
	}
	if containerStat.Logs != nil {
		fStat := convertFileSystemStats(*containerStat.Logs)
		fStat.Name = "logfs"
		rawMetic.FileSystemStats = append(rawMetic.FileSystemStats, fStat)
	}

	return rawMetic
}

// ConvertNodeToRaw Converts Kubelet Node stats to RawMetric.
func ConvertNodeToRaw(nodeStat stats.NodeStats) RawMetric {
	var rawMetic RawMetric

	rawMetic.Id = nodeStat.NodeName
	rawMetic.Name = nodeStat.NodeName

	if nodeStat.CPU != nil {
		rawMetic.Time = nodeStat.CPU.Time.Time
		rawMetic.CPUStats = convertCPUStats(*nodeStat.CPU)
	}

	if nodeStat.Memory != nil {
		if rawMetic.Time.IsZero() {
			rawMetic.Time = nodeStat.Memory.Time.Time
		}
		rawMetic.MemoryStats = convertMemoryStats(*nodeStat.Memory)
	}

	rawMetic.FileSystemStats = []FileSystemStat{}
	if nodeStat.Fs != nil {
		rawMetic.FileSystemStats = append(rawMetic.FileSystemStats, convertFileSystemStats(*nodeStat.Fs))
	}

	if nodeStat.Network != nil {
		for _, intfStats := range nodeStat.Network.Interfaces {
			rawMetic.NetworkStats = append(rawMetic.NetworkStats, convertNetworkStats(*nodeStat.Network, intfStats))
		}
	}

	return rawMetic
}

// ConvertHCSContainerToRaw Converts HCS Container stats to RawMetric.
func ConvertHCSContainerToRaw(containerStat HCSStat) RawMetric {
	var rawMetic RawMetric

	rawMetic.Id = containerStat.Id
	rawMetic.Name = containerStat.Name
	rawMetic.Time = containerStat.Time

	if containerStat.Network != nil {
		for _, val := range *containerStat.Network {
			rawMetic.NetworkStats = append(rawMetic.NetworkStats, convertHCSNetworkStats(containerStat, val))
		}
	}

	return rawMetic
}
