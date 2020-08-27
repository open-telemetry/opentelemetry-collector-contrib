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

import (
	"time"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
)

func taskMetrics(prefix string, stats *TaskStats, taskLimit Limit) []*metricspb.Metric {
	return applyCurrentTime([]*metricspb.Metric{
		memUsageMetric(prefix, stats.MemoryUsage),
		memMaxUsageMetric(prefix, stats.MemoryMaxUsage),
		memLimitMetric(prefix, stats.MemoryLimit),
		memUtilizedMetric(prefix, stats.MemoryUtilized),
		rxBytesPerSecond(prefix, stats.NetworkRateRxBytesPerSecond),
		txBytesPerSecond(prefix, stats.NetworkRateTxBytesPerSecond),
		rxBytes(prefix, stats.NetworkRxBytes),
		rxPackets(prefix, stats.NetworkRxPackets),
		rxErrors(prefix, stats.NetworkRxErrors),
		rxDropped(prefix, stats.NetworkRxDropped),
		txBytes(prefix, stats.NetworkTxBytes),
		txPackets(prefix, stats.NetworkTxPackets),
		txErrors(prefix, stats.NetworkTxErrors),
		txDropped(prefix, stats.NetworkTxDropped),
		totalUsageMetric(prefix, stats.CPUTotalUsage),
		usageInKernelMode(prefix, stats.CPUUsageInKernelmode),
		usageInUserMode(prefix, stats.CPUUsageInUserMode),
		numberOfCores(prefix, stats.NumOfCPUCores),
		onlineCpus(prefix, stats.CPUOnlineCpus),
		systemCpuUsage(prefix, stats.SystemCPUUsage),
		storageReadBytes(prefix, stats.StorageReadBytes),
		storageWriteBytes(prefix, stats.StorageWriteBytes),
		memReserved(prefix, taskLimit.Memory),
		cpuReserved(prefix, taskLimit.CPU),
	}, time.Now())
}
