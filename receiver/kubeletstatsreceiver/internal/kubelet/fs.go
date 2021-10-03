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

func addFilesystemMetrics(dest pdata.MetricSlice, prefix string, s *stats.FsStats, currentTime pdata.Timestamp) {
	if s == nil {
		return
	}

	addIntGauge(dest, prefix, metadata.M.FilesystemAvailable, s.AvailableBytes, currentTime)
	addIntGauge(dest, prefix, metadata.M.FilesystemCapacity, s.CapacityBytes, currentTime)
	addIntGauge(dest, prefix, metadata.M.FilesystemUsage, s.UsedBytes, currentTime)
}
