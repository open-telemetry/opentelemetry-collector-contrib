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
	"go.opentelemetry.io/collector/model/pdata"
	stats "k8s.io/kubelet/pkg/apis/stats/v1alpha1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/metadata"
)

func addCPUMetrics(dest pdata.MetricSlice, prefix string, s *stats.CPUStats, startTime pdata.Timestamp, currentTime pdata.Timestamp) {
	if s == nil {
		return
	}
	addCPUUsageMetric(dest, prefix, s, currentTime)
	addCPUTimeMetric(dest, prefix, s, startTime, currentTime)
}

func addCPUUsageMetric(dest pdata.MetricSlice, prefix string, s *stats.CPUStats, currentTime pdata.Timestamp) {
	if s.UsageNanoCores == nil {
		return
	}
	value := float64(*s.UsageNanoCores) / 1_000_000_000
	fillDoubleGauge(dest.AppendEmpty(), prefix, metadata.M.CPUUtilization, value, currentTime)
}

func addCPUTimeMetric(dest pdata.MetricSlice, prefix string, s *stats.CPUStats, startTime pdata.Timestamp, currentTime pdata.Timestamp) {
	if s.UsageCoreNanoSeconds == nil {
		return
	}
	value := float64(*s.UsageCoreNanoSeconds) / 1_000_000_000
	fillDoubleSum(dest.AppendEmpty(), prefix, metadata.M.CPUTime, value, startTime, currentTime)
}
