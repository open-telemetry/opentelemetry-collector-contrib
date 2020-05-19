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

func fsMetrics(prefix string, s *stats.FsStats) []*metricspb.Metric {
	return applyCurrentTime([]*metricspb.Metric{
		fsAvailableMetric(prefix, s),
		fsCapacityMetric(prefix, s),
	}, s.Time.Time)
}

func fsAvailableMetric(prefix string, s *stats.FsStats) *metricspb.Metric {
	return intGauge(prefix+"fs/available", "By", s.AvailableBytes)
}

func fsCapacityMetric(prefix string, s *stats.FsStats) *metricspb.Metric {
	return intGauge(prefix+"fs/capacity", "By", s.CapacityBytes)
}
