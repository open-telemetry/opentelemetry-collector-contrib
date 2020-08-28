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

func cpuMetrics(prefix string, stats *CPUStats, containerMetadata ContainerMetadata, labelKeys []*metricspb.LabelKey, labelValues []*metricspb.LabelValue) []*metricspb.Metric {
	numOfCores := (uint64)(len(stats.CpuUsage.PerCpuUsage))
	utilized := (uint64)(*stats.CpuUsage.TotalUsage / numOfCores / 1024)
	cpuLimit := (float64)(*containerMetadata.Limits.CPU / 1024)
	return applyCurrentTime([]*metricspb.Metric{
		intGauge(prefix+"cpu.total_usage", "Count", stats.CpuUsage.TotalUsage, labelKeys, labelValues),
		intGauge(prefix+"cpu.usage_in_kernelmode", "Count", stats.CpuUsage.UsageInKernelmode, labelKeys, labelValues),
		intGauge(prefix+"cpu.usage_in_usermode", "Count", stats.CpuUsage.UsageInUserMode, labelKeys, labelValues),
		intGauge(prefix+"cpu.number_of_cores", "Count", &numOfCores, labelKeys, labelValues),
		intGauge(prefix+"cpu.online_cpus", "Count", stats.OnlineCpus, labelKeys, labelValues),
		intGauge(prefix+"cpu.system_cpu_usage", "Count", stats.SystemCpuUsage, labelKeys, labelValues),
		intGauge(prefix+"cpu.cpu_utilized", "Count", &utilized, labelKeys, labelValues),
		doubleGauge(prefix+"cpu.cpu_reserved", "vCPU", &cpuLimit, labelKeys, labelValues),
	}, time.Now())
}

// func totalUsageMetric(prefix string, value *uint64) *metricspb.Metric {
// 	return intGauge(prefix+"cpu.total_usage", "Count", value)
// }

// func usageInKernelMode(prefix string, value *uint64) *metricspb.Metric {
// 	return intGauge(prefix+"cpu.usage_in_kernelmode", "Count", value)
// }

// func usageInUserMode(prefix string, value *uint64) *metricspb.Metric {
// 	return intGauge(prefix+"cpu.usage_in_usermode", "Count", value)
// }

// func numberOfCores(prefix string, value *uint64) *metricspb.Metric {
// 	return intGauge(prefix+"cpu.number_of_cores", "Count", value)
// }

// func onlineCpus(prefix string, value *uint64) *metricspb.Metric {
// 	return intGauge(prefix+"cpu.online_cpus", "Count", value)
// }

// func systemCpuUsage(prefix string, value *uint64) *metricspb.Metric {
// 	return intGauge(prefix+"cpu.system_cpu_usage", "Count", value)
// }

// func cpuUtilized(prefix string, value *uint64) *metricspb.Metric {
// 	return intGauge(prefix+"cpu.cpu_utilized", "Count", value)
// }

// func cpuReserved(prefix string, value *float64) *metricspb.Metric {
// 	return doubleGauge(prefix+"cpu.cpu_reserved", "vCPU", value)
// }
