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

func addMemoryMetrics(dest pdata.MetricSlice, prefix string, s *stats.MemoryStats, currentTime pdata.Timestamp) {
	if s == nil {
		return
	}

	addIntGauge(dest, prefix, metadata.M.MemoryAvailable, s.AvailableBytes, currentTime)
	addIntGauge(dest, prefix, metadata.M.MemoryUsage, s.UsageBytes, currentTime)
	addIntGauge(dest, prefix, metadata.M.MemoryRss, s.RSSBytes, currentTime)
	addIntGauge(dest, prefix, metadata.M.MemoryWorkingSet, s.WorkingSetBytes, currentTime)
	addIntGauge(dest, prefix, metadata.M.MemoryPageFaults, s.PageFaults, currentTime)
	addIntGauge(dest, prefix, metadata.M.MemoryMajorPageFaults, s.MajorPageFaults, currentTime)
}
