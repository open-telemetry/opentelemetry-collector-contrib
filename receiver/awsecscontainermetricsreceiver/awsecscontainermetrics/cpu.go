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

func cpuMetrics(prefix string, stats *CPUStats) []*metricspb.Metric {
	return applyCurrentTime([]*metricspb.Metric{
		totalUsageMetric(prefix, stats),
		usageInKernelMode(prefix, stats),
		usageInUserMode(prefix, stats),
		numberOfCores(prefix, stats),
		onlineCpus(prefix, stats),
		systemCpuUsage(prefix, stats),
	}, time.Now())
}

func totalUsageMetric(prefix string, s *CPUStats) *metricspb.Metric {
	return intGauge(prefix+"cpu.total_usage", "Count", s.CpuUsage.TotalUsage)
}

func usageInKernelMode(prefix string, s *CPUStats) *metricspb.Metric {
	return intGauge(prefix+"cpu.usage_in_kernelmode", "Count", s.CpuUsage.UsageInKernelmode)
}

func usageInUserMode(prefix string, s *CPUStats) *metricspb.Metric {
	return intGauge(prefix+"cpu.usage_in_usermode", "Count", s.CpuUsage.UsageInUserMode)
}

func numberOfCores(prefix string, s *CPUStats) *metricspb.Metric {
	numOfCores := (uint64)(len(s.CpuUsage.PerCpuUsage))
	return intGauge(prefix+"cpu.number_of_cores", "Count", &numOfCores)
}

func onlineCpus(prefix string, s *CPUStats) *metricspb.Metric {
	return intGauge(prefix+"cpu.online_cpus", "Count", s.OnlineCpus)
}

func systemCpuUsage(prefix string, s *CPUStats) *metricspb.Metric {
	return intGauge(prefix+"cpu.system_cpu_usage", "Count", s.SystemCpuUsage)
}
