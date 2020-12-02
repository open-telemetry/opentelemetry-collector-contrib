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

func memMetrics(prefix string, s *stats.MemoryStats) []*metricspb.Metric {
	if s == nil {
		return nil
	}
	return []*metricspb.Metric{
		memAvailableMetric(prefix, s),
		memUsageMetric(prefix, s),
		memRssMetric(prefix, s),
		memWorkingSetMetric(prefix, s),
		memPageFaultsMetric(prefix, s),
		memMajorPageFaultsMetric(prefix, s),
	}
}

func memAvailableMetric(prefix string, s *stats.MemoryStats) *metricspb.Metric {
	return intGauge(prefix+"memory.available", "By", s.AvailableBytes)
}

func memUsageMetric(prefix string, s *stats.MemoryStats) *metricspb.Metric {
	return intGauge(prefix+"memory.usage", "By", s.UsageBytes)
}

func memRssMetric(prefix string, s *stats.MemoryStats) *metricspb.Metric {
	return intGauge(prefix+"memory.rss", "By", s.RSSBytes)
}

func memWorkingSetMetric(prefix string, s *stats.MemoryStats) *metricspb.Metric {
	return intGauge(prefix+"memory.working_set", "By", s.WorkingSetBytes)
}

func memPageFaultsMetric(prefix string, s *stats.MemoryStats) *metricspb.Metric {
	return intGauge(prefix+"memory.page_faults", "1", s.PageFaults)
}

func memMajorPageFaultsMetric(prefix string, s *stats.MemoryStats) *metricspb.Metric {
	return intGauge(prefix+"memory.major_page_faults", "1", s.MajorPageFaults)
}
