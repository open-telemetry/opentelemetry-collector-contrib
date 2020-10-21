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

package kubelet

import (
	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	stats "k8s.io/kubernetes/pkg/kubelet/apis/stats/v1alpha1"
)

func cpuMetrics(prefix string, s *stats.CPUStats) []*metricspb.Metric {
	if s == nil {
		return nil
	}
	return []*metricspb.Metric{
		cpuUsageMetric(prefix, s),
		cpuCumulativeUsageMetric(prefix, s),
	}
}

func cpuUsageMetric(prefix string, s *stats.CPUStats) *metricspb.Metric {
	nanoCores := s.UsageNanoCores
	if nanoCores == nil {
		return nil
	}
	value := float64(*nanoCores) / 1_000_000_000
	return doubleGauge(prefix+"cpu.utilization", "1", &value)
}

func cpuCumulativeUsageMetric(prefix string, s *stats.CPUStats) *metricspb.Metric {
	value := float64(*s.UsageCoreNanoSeconds) / 1_000_000_000
	return cumulativeDouble(prefix+"cpu.time", "s", &value)
}
