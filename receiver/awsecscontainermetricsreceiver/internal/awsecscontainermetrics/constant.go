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

package awsecscontainermetrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsecscontainermetricsreceiver/internal/awsecscontainermetrics"

// Constant attributes for aws ecs container metrics.
const (
	attributeECSDockerName        = "aws.ecs.docker.name"
	attributeECSCluster           = "aws.ecs.cluster.name"
	attributeECSTaskID            = "aws.ecs.task.id"
	attributeECSTaskRevision      = "aws.ecs.task.version"
	attributeECSServiceName       = "aws.ecs.service.name"
	attributeECSTaskPullStartedAt = "aws.ecs.task.pull_started_at"
	attributeECSTaskPullStoppedAt = "aws.ecs.task.pull_stopped_at"
	attributeECSTaskKnownStatus   = "aws.ecs.task.known_status"
	attributeECSTaskLaunchType    = "aws.ecs.task.launch_type"
	attributeContainerImageID     = "aws.ecs.container.image.id"
	attributeContainerCreatedAt   = "aws.ecs.container.created_at"
	attributeContainerStartedAt   = "aws.ecs.container.started_at"
	attributeContainerFinishedAt  = "aws.ecs.container.finished_at"
	attributeContainerKnownStatus = "aws.ecs.container.know_status"
	attributeContainerExitCode    = "aws.ecs.container.exit_code"

	cpusInVCpu = 1024
	bytesInMiB = 1024 * 1024

	taskPrefix      = "ecs.task."
	containerPrefix = "container."

	attributeMemoryUsage    = "memory.usage"
	attributeMemoryMaxUsage = "memory.usage.max"
	attributeMemoryLimit    = "memory.usage.limit"
	attributeMemoryReserved = "memory.reserved"
	attributeMemoryUtilized = "memory.utilized"

	attributeCPUTotalUsage      = "cpu.usage.total"
	attributeCPUKernelModeUsage = "cpu.usage.kernelmode"
	attributeCPUUserModeUsage   = "cpu.usage.usermode"
	attributeCPUSystemUsage     = "cpu.usage.system"
	attributeCPUCores           = "cpu.cores"
	attributeCPUOnlines         = "cpu.onlines"
	attributeCPUReserved        = "cpu.reserved"
	attributeCPUUtilized        = "cpu.utilized"
	attributeCPUUsageInVCPU     = "cpu.usage.vcpu"

	attributeNetworkRateRx = "network.rate.rx"
	attributeNetworkRateTx = "network.rate.tx"

	attributeNetworkRxBytes   = "network.io.usage.rx_bytes"
	attributeNetworkRxPackets = "network.io.usage.rx_packets"
	attributeNetworkRxErrors  = "network.io.usage.rx_errors"
	attributeNetworkRxDropped = "network.io.usage.rx_dropped"
	attributeNetworkTxBytes   = "network.io.usage.tx_bytes"
	attributeNetworkTxPackets = "network.io.usage.tx_packets"
	attributeNetworkTxErrors  = "network.io.usage.tx_errors"
	attributeNetworkTxDropped = "network.io.usage.tx_dropped"

	attributeStorageRead  = "storage.read_bytes"
	attributeStorageWrite = "storage.write_bytes"

	attributeDuration = "duration"

	unitBytes       = "Bytes"
	unitMegaBytes   = "Megabytes"
	unitNanoSecond  = "Nanoseconds"
	unitBytesPerSec = "Bytes/Second"
	unitCount       = "Count"
	unitVCpu        = "vCPU"
	unitSecond      = "Seconds"
	unitNone        = "None"
)
