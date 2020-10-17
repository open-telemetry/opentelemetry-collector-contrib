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

package awsecscontainermetrics

const (
	AttributeECSDockerName   = "ecs.docker-name"
	AttributeECSCluster      = "ecs.cluster"
	AttributeECSTaskARN      = "ecs.task-arn"
	AttributeECSTaskID       = "ecs.task-id"
	AttributeECSTaskFamily   = "ecs.task-definition-family"
	AttributeECSTaskRevesion = "ecs.task-definition-version"
	AttributeECSServiceName  = "ecs.service"

	CPUsInVCpu = 1024
	BytesInMiB = 1024 * 1024

	TaskPrefix         = "ecs.task."
	ContainerPrefix    = "container."
	MetricResourceType = "aoc.ecs"

	EndpointEnvKey   = "ECS_CONTAINER_METADATA_URI_V4"
	TaskStatsPath    = "/task/stats"
	TaskMetadataPath = "/task"

	AttributeMemoryUsage    = "memory.usage"
	AttributeMemoryMaxUsage = "memory.usage.max"
	AttributeMemoryLimit    = "memory.usage.limit"
	AttributeMemoryReserved = "memory.reserved"
	AttributeMemoryUtilized = "memory.utilized"

	AttributeCPUTotalUsage      = "cpu.usage.total"
	AttributeCPUKernelModeUsage = "cpu.usage.kernelmode"
	AttributeCPUUserModeUsage   = "cpu.usage.usermode"
	AttributeCPUSystemUsage     = "cpu.usage.system"
	AttributeCPUCores           = "cpu.cores"
	AttributeCPUOnlines         = "cpu.onlines"
	AttributeCPUReserved        = "cpu.reserved"
	AttributeCPUUtilized        = "cpu.utilized"
	AttributeCPUUsageInVCPU     = "cpu.usage.vcpu"

	AttributeNetworkRateRx = "network.rate.rx"
	AttributeNetworkRateTx = "network.rate.tx"

	AttributeNetworkRxBytes   = "network.io.usage.rx_bytes"
	AttributeNetworkRxPackets = "network.io.usage.rx_packets"
	AttributeNetworkRxErrors  = "network.io.usage.rx_errors"
	AttributeNetworkRxDropped = "network.io.usage.rx_dropped"
	AttributeNetworkTxBytes   = "network.io.usage.tx_bytes"
	AttributeNetworkTxPackets = "network.io.usage.tx_packets"
	AttributeNetworkTxErrors  = "network.io.usage.tx_errors"
	AttributeNetworkTxDropped = "network.io.usage.tx_dropped"

	AttributeStorageRead  = "storage.read_bytes"
	AttributeStorageWrite = "storage.write_bytes"

	UnitBytes       = "Bytes"
	UnitMegaBytes   = "MB"
	UnitNanoSecond  = "NS"
	UnitBytesPerSec = "Bytes/Sec"
	UnitCount       = "Count"
	UnitVCpu        = "vCPU"
	UnitPercent     = "Percent"
)
