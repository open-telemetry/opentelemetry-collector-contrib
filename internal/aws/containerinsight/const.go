// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package containerinsight // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight"

import (
	"strings"
	"time"
)

// define metric names, attribute names, metric types, and units for both EKS and ECS Container Insights
const (
	// We assume 50 micro-seconds is the minimal gap between two collected data sample to be valid to calculate delta
	MinTimeDiff = 50 * time.Microsecond

	// Environment variables
	HostName                  = "HOST_NAME"
	HostIP                    = "HOST_IP"
	RunInContainer            = "RUN_IN_CONTAINER"
	RunAsHostProcessContainer = "RUN_AS_HOST_PROCESS_CONTAINER"

	// Attribute names
	InstanceID              = "InstanceId"
	InstanceType            = "InstanceType"
	ClusterNameKey          = "ClusterName"
	AutoScalingGroupNameKey = "AutoScalingGroupName"
	NodeNameKey             = "NodeName"
	Version                 = "Version"
	DiskDev                 = "device"
	EbsVolumeID             = "ebs_volume_id" // used by kubernetes cluster as persistent volume
	HostEbsVolumeID         = "EBSVolumeId"   // used by host filesystem
	FSType                  = "fstype"
	MetricType              = "Type"
	SourcesKey              = "Sources"
	Timestamp               = "Timestamp"
	OperatingSystem         = "OperatingSystem"
	OperatingSystemWindows  = "windows"

	// The following constants are used for metric name construction
	CPUTotal                         = "cpu_usage_total"
	CPUUser                          = "cpu_usage_user"
	CPUSystem                        = "cpu_usage_system"
	CPULimit                         = "cpu_limit"
	CPUUtilization                   = "cpu_utilization"
	CPURequest                       = "cpu_request"
	CPUReservedCapacity              = "cpu_reserved_capacity"
	CPUUtilizationOverPodLimit       = "cpu_utilization_over_pod_limit"
	CPUUtilizationOverContainerLimit = "cpu_utilization_over_container_limit"

	MemUsage                         = "memory_usage"
	MemCache                         = "memory_cache"
	MemRss                           = "memory_rss"
	MemMaxusage                      = "memory_max_usage"
	MemSwap                          = "memory_swap"
	MemFailcnt                       = "memory_failcnt"
	MemMappedfile                    = "memory_mapped_file"
	MemWorkingset                    = "memory_working_set"
	MemPgfault                       = "memory_pgfault"
	MemPgmajfault                    = "memory_pgmajfault"
	MemFailuresTotal                 = "memory_failures_total"
	MemHierarchicalPgfault           = "memory_hierarchical_pgfault"
	MemHierarchicalPgmajfault        = "memory_hierarchical_pgmajfault"
	MemLimit                         = "memory_limit"
	MemRequest                       = "memory_request"
	MemUtilization                   = "memory_utilization"
	MemReservedCapacity              = "memory_reserved_capacity"
	MemUtilizationOverPodLimit       = "memory_utilization_over_pod_limit"
	MemUtilizationOverContainerLimit = "memory_utilization_over_container_limit"

	NetIfce       = "interface"
	NetRxBytes    = "network_rx_bytes"
	NetRxPackets  = "network_rx_packets"
	NetRxDropped  = "network_rx_dropped"
	NetRxErrors   = "network_rx_errors"
	NetTxBytes    = "network_tx_bytes"
	NetTxPackets  = "network_tx_packets"
	NetTxDropped  = "network_tx_dropped"
	NetTxErrors   = "network_tx_errors"
	NetTotalBytes = "network_total_bytes"

	FSUsage       = "filesystem_usage"
	FSCapacity    = "filesystem_capacity"
	FSAvailable   = "filesystem_available"
	FSInodes      = "filesystem_inodes"
	FSInodesfree  = "filesystem_inodes_free"
	FSUtilization = "filesystem_utilization"

	StatusConditionReady                                   = "status_condition_ready"
	StatusConditionDiskPressure                            = "status_condition_disk_pressure"
	StatusConditionMemoryPressure                          = "status_condition_memory_pressure"
	StatusConditionPIDPressure                             = "status_condition_pid_pressure"
	StatusConditionNetworkUnavailable                      = "status_condition_network_unavailable"
	StatusConditionUnknown                                 = "status_condition_unknown"
	StatusCapacityPods                                     = "status_capacity_pods"
	StatusAllocatablePods                                  = "status_allocatable_pods"
	StatusNumberAvailable                                  = "status_number_available"
	StatusNumberUnavailable                                = "status_number_unavailable"
	StatusDesiredNumberScheduled                           = "status_desired_number_scheduled"
	StatusCurrentNumberScheduled                           = "status_current_number_scheduled"
	StatusReplicasAvailable                                = "status_replicas_available"
	StatusReplicasUnavailable                              = "status_replicas_unavailable"
	SpecReplicas                                           = "spec_replicas"
	StatusContainerRunning                                 = "container_status_running"
	StatusContainerTerminated                              = "container_status_terminated"
	StatusContainerWaiting                                 = "container_status_waiting"
	StatusContainerWaitingReasonCrashLoopBackOff           = "container_status_waiting_reason_crash_loop_back_off"
	StatusContainerWaitingReasonImagePullError             = "container_status_waiting_reason_image_pull_error"
	StatusContainerWaitingReasonStartError                 = "container_status_waiting_reason_start_error"
	StatusContainerWaitingReasonCreateContainerError       = "container_status_waiting_reason_create_container_error"
	StatusContainerWaitingReasonCreateContainerConfigError = "container_status_waiting_reason_create_container_config_error"
	StatusContainerTerminatedReasonOOMKilled               = "container_status_terminated_reason_oom_killed"
	StatusRunning                                          = "status_running"
	StatusPending                                          = "status_pending"
	StatusSucceeded                                        = "status_succeeded"
	StatusFailed                                           = "status_failed"
	StatusUnknown                                          = "status_unknown"
	StatusReady                                            = "status_ready"
	StatusScheduled                                        = "status_scheduled"
	ReplicasDesired                                        = "replicas_desired"
	ReplicasReady                                          = "replicas_ready"

	RunningPodCount       = "number_of_running_pods"
	RunningContainerCount = "number_of_running_containers"
	ContainerCount        = "number_of_containers"
	NodeCount             = "node_count"
	FailedNodeCount       = "failed_node_count"
	ContainerRestartCount = "number_of_container_restarts"
	RunningTaskCount      = "number_of_running_tasks"

	DiskIOServiceBytesPrefix = "diskio_io_service_bytes_"
	DiskIOServicedPrefix     = "diskio_io_serviced_"
	DiskIOAsync              = "Async"
	DiskIORead               = "Read"
	DiskIOSync               = "Sync"
	DiskIOWrite              = "Write"
	DiskIOTotal              = "Total"

	EfaRdmaReadBytes      = "rdma_read_bytes"
	EfaRdmaWriteBytes     = "rdma_write_bytes"
	EfaRdmaWriteRecvBytes = "rdma_write_recv_bytes"
	EfaRxBytes            = "rx_bytes"
	EfaRxDropped          = "rx_dropped"
	EfaTxBytes            = "tx_bytes"

	GpuLimit            = "gpu_limit"
	GpuUsageTotal       = "gpu_usage_total"
	GpuRequest          = "gpu_request"
	GpuReservedCapacity = "gpu_reserved_capacity"

	NeuroncoreLimit              = "neuroncore_limit"
	NeuroncoreUsageTotal         = "neuroncore_usage_total"
	NeuroncoreRequest            = "neuroncore_request"
	NeuroncoreReservedCapacity   = "neuroncore_reserved_capacity"
	NeuroncoreUnreservedCapacity = "neuroncore_unreserved_capacity"
	NeuroncoreAvailableCapacity  = "neuroncore_available_capacity"

	HyperPodUnschedulablePendingReplacement = "unschedulable_pending_replacement"
	HyperPodUnschedulablePendingReboot      = "unschedulable_pending_reboot"
	HyperPodSchedulable                     = "schedulable"
	HyperPodUnschedulable                   = "unschedulable"

	// kueue metrics

	KueuePendingWorkloads          = "kueue_pending_workloads"
	KueueEvictedWorkloadsTotal     = "kueue_evicted_workloads_total"
	KueueAdmittedActiveWorkloads   = "kueue_admitted_active_workloads"
	KueueClusterQueueResourceUsage = "kueue_cluster_queue_resource_usage"
	KueueClusterQueueNominalQuota  = "kueue_cluster_queue_nominal_quota"

	// Define the metric types
	TypeCluster            = "Cluster"
	TypeClusterService     = "ClusterService"
	TypeClusterDeployment  = "ClusterDeployment"
	TypeClusterDaemonSet   = "ClusterDaemonSet"
	TypeClusterStatefulSet = "ClusterStatefulSet"
	TypeClusterReplicaSet  = "ClusterReplicaSet"
	TypeClusterNamespace   = "ClusterNamespace"
	TypeService            = "Service"
	TypeInstance           = "Instance" // mean EC2 Instance in ECS
	TypeNode               = "Node"     // mean EC2 Instance in EKS
	TypeInstanceFS         = "InstanceFS"
	TypeNodeFS             = "NodeFS"
	TypeInstanceNet        = "InstanceNet"
	TypeNodeNet            = "NodeNet"
	TypeInstanceDiskIO     = "InstanceDiskIO"
	TypeNodeDiskIO         = "NodeDiskIO"
	TypePod                = "Pod"
	TypePodNet             = "PodNet"
	TypeContainer          = "Container"
	TypeContainerFS        = "ContainerFS"
	TypeContainerDiskIO    = "ContainerDiskIO"

	// kueue metric types
	TypeClusterQueue = "ClusterQueue"

	// Special type for pause container
	// because containerd does not set container name pause container name to POD like docker does.
	TypeInfraContainer  = "InfraContainer"
	TypeContainerGPU    = "ContainerGPU"
	TypePodGPU          = "PodGPU"
	TypeNodeGPU         = "NodeGPU"
	TypeClusterGPU      = "ClusterGPU"
	TypeContainerNeuron = "ContainerNeuron"
	TypeContainerEFA    = "ContainerEFA"
	TypePodEFA          = "PodEFA"
	TypeNodeEFA         = "NodeEFA"
	TypeHyperPodNode    = "HyperPodNode"
	TypeNodeNVME        = "NodeNVME"

	// unit
	UnitBytes       = "Bytes"
	UnitMegaBytes   = "Megabytes"
	UnitSecond      = "Second"
	UnitNanoSecond  = "Nanoseconds"
	UnitBytesPerSec = "Bytes/Second"
	UnitCount       = "Count"
	UnitCountPerSec = "Count/Second"
	UnitVCPU        = "vCPU"
	UnitPercent     = "Percent"
	TrueValue       = "True"
)

var WaitingReasonLookup = map[string]string{
	"CrashLoopBackOff":           StatusContainerWaitingReasonCrashLoopBackOff,
	"ErrImagePull":               StatusContainerWaitingReasonImagePullError,
	"ImagePullBackOff":           StatusContainerWaitingReasonImagePullError,
	"InvalidImageName":           StatusContainerWaitingReasonImagePullError,
	"CreateContainerError":       StatusContainerWaitingReasonCreateContainerError,
	"CreateContainerConfigError": StatusContainerWaitingReasonCreateContainerConfigError,
	"StartError":                 StatusContainerWaitingReasonStartError,
}

var HyperPodConditionToMetric = map[string]string{
	"UnschedulablePendingReplacement": HyperPodUnschedulablePendingReplacement,
	"UnschedulablePendingReboot":      HyperPodUnschedulablePendingReboot,
	"Schedulable":                     HyperPodSchedulable,
	"Unschedulable":                   HyperPodUnschedulable,
}

var metricToUnitMap map[string]string

func init() {
	metricToUnitMap = map[string]string{
		// cpu metrics
		// The following metrics are reported in unit of millicores, but cloudwatch doesn't support it
		// CPUTotal
		// CPUUser
		// CPUSystem
		// CPULimit
		// CPURequest
		CPUUtilization:                   UnitPercent,
		CPUReservedCapacity:              UnitPercent,
		CPUUtilizationOverPodLimit:       UnitPercent,
		CPUUtilizationOverContainerLimit: UnitPercent,

		// memory metrics
		MemUsage:                         UnitBytes,
		MemCache:                         UnitBytes,
		MemRss:                           UnitBytes,
		MemMaxusage:                      UnitBytes,
		MemSwap:                          UnitBytes,
		MemFailcnt:                       UnitCount,
		MemMappedfile:                    UnitBytes,
		MemWorkingset:                    UnitBytes,
		MemRequest:                       UnitBytes,
		MemLimit:                         UnitBytes,
		MemUtilization:                   UnitPercent,
		MemReservedCapacity:              UnitPercent,
		MemUtilizationOverPodLimit:       UnitPercent,
		MemUtilizationOverContainerLimit: UnitPercent,

		MemPgfault:                UnitCountPerSec,
		MemPgmajfault:             UnitCountPerSec,
		MemFailuresTotal:          UnitCountPerSec,
		MemHierarchicalPgfault:    UnitCountPerSec,
		MemHierarchicalPgmajfault: UnitCountPerSec,

		// disk io metrics
		strings.ToLower(DiskIOServiceBytesPrefix + DiskIOAsync): UnitBytesPerSec,
		strings.ToLower(DiskIOServiceBytesPrefix + DiskIORead):  UnitBytesPerSec,
		strings.ToLower(DiskIOServiceBytesPrefix + DiskIOSync):  UnitBytesPerSec,
		strings.ToLower(DiskIOServiceBytesPrefix + DiskIOWrite): UnitBytesPerSec,
		strings.ToLower(DiskIOServiceBytesPrefix + DiskIOTotal): UnitBytesPerSec,
		strings.ToLower(DiskIOServicedPrefix + DiskIOAsync):     UnitCountPerSec,
		strings.ToLower(DiskIOServicedPrefix + DiskIORead):      UnitCountPerSec,
		strings.ToLower(DiskIOServicedPrefix + DiskIOSync):      UnitCountPerSec,
		strings.ToLower(DiskIOServicedPrefix + DiskIOWrite):     UnitCountPerSec,
		strings.ToLower(DiskIOServicedPrefix + DiskIOTotal):     UnitCountPerSec,

		// network metrics
		NetRxBytes:    UnitBytesPerSec,
		NetRxPackets:  UnitCountPerSec,
		NetRxDropped:  UnitCountPerSec,
		NetRxErrors:   UnitCountPerSec,
		NetTxBytes:    UnitBytesPerSec,
		NetTxPackets:  UnitCountPerSec,
		NetTxDropped:  UnitCountPerSec,
		NetTxErrors:   UnitCountPerSec,
		NetTotalBytes: UnitBytesPerSec,

		// filesystem metrics
		FSUsage:       UnitBytes,
		FSCapacity:    UnitBytes,
		FSAvailable:   UnitBytes,
		FSInodes:      UnitCount,
		FSInodesfree:  UnitCount,
		FSUtilization: UnitPercent,

		// status & spec metrics
		StatusConditionReady:              UnitCount,
		StatusConditionDiskPressure:       UnitCount,
		StatusConditionMemoryPressure:     UnitCount,
		StatusConditionPIDPressure:        UnitCount,
		StatusConditionNetworkUnavailable: UnitCount,
		StatusConditionUnknown:            UnitCount,
		StatusCapacityPods:                UnitCount,
		StatusAllocatablePods:             UnitCount,
		StatusReplicasAvailable:           UnitCount,
		StatusReplicasUnavailable:         UnitCount,
		StatusNumberAvailable:             UnitCount,
		StatusNumberUnavailable:           UnitCount,
		StatusDesiredNumberScheduled:      UnitCount,
		StatusCurrentNumberScheduled:      UnitCount,
		SpecReplicas:                      UnitCount,
		ReplicasDesired:                   UnitCount,
		ReplicasReady:                     UnitCount,

		// kube-state-metrics equivalents
		StatusContainerRunning:                                 UnitCount,
		StatusContainerTerminated:                              UnitCount,
		StatusContainerWaiting:                                 UnitCount,
		StatusContainerWaitingReasonCrashLoopBackOff:           UnitCount,
		StatusContainerWaitingReasonImagePullError:             UnitCount,
		StatusContainerWaitingReasonStartError:                 UnitCount,
		StatusContainerWaitingReasonCreateContainerConfigError: UnitCount,
		StatusContainerWaitingReasonCreateContainerError:       UnitCount,
		StatusContainerTerminatedReasonOOMKilled:               UnitCount,
		StatusRunning:                                          UnitCount,
		StatusFailed:                                           UnitCount,
		StatusPending:                                          UnitCount,
		StatusSucceeded:                                        UnitCount,
		StatusUnknown:                                          UnitCount,
		StatusReady:                                            UnitCount,
		StatusScheduled:                                        UnitCount,

		// cluster metrics
		NodeCount:       UnitCount,
		FailedNodeCount: UnitCount,

		// kueue metrics
		KueuePendingWorkloads:          UnitCount,
		KueueEvictedWorkloadsTotal:     UnitCount,
		KueueAdmittedActiveWorkloads:   UnitCount,
		KueueClusterQueueResourceUsage: UnitCount,
		KueueClusterQueueNominalQuota:  UnitCount,
		// unit for KueueClusterQueue resource metrics depend on resource type. UnitCount is appropriate
		// for CPU and CPU cores, but UnitBytes would be more appropriate for resource type memory.

		// others
		RunningPodCount:       UnitCount,
		RunningContainerCount: UnitCount,
		ContainerCount:        UnitCount,
		ContainerRestartCount: UnitCount,
		RunningTaskCount:      UnitCount,

		EfaRdmaReadBytes:      UnitBytesPerSec,
		EfaRdmaWriteBytes:     UnitBytesPerSec,
		EfaRdmaWriteRecvBytes: UnitBytesPerSec,
		EfaRxBytes:            UnitBytesPerSec,
		EfaRxDropped:          UnitCountPerSec,
		EfaTxBytes:            UnitBytesPerSec,

		GpuLimit:            UnitCount,
		GpuUsageTotal:       UnitCount,
		GpuRequest:          UnitCount,
		GpuReservedCapacity: UnitPercent,

		NeuroncoreLimit:              UnitCount,
		NeuroncoreUsageTotal:         UnitCount,
		NeuroncoreRequest:            UnitCount,
		NeuroncoreReservedCapacity:   UnitPercent,
		NeuroncoreUnreservedCapacity: UnitPercent,
		NeuroncoreAvailableCapacity:  UnitCount,

		HyperPodUnschedulablePendingReplacement: UnitCount,
		HyperPodUnschedulablePendingReboot:      UnitCount,
		HyperPodSchedulable:                     UnitCount,
		HyperPodUnschedulable:                   UnitCount,
	}
}
