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

package kubelet // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/kubelet"
import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	stats "k8s.io/kubelet/pkg/apis/stats/v1alpha1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/metadata"
)

func addNetworkMetrics(mb *metadata.MetricsBuilder, networkMetrics metadata.NetworkMetrics, s *stats.NetworkStats, currentTime pcommon.Timestamp) {
	if s == nil {
		return
	}

	recordNetworkDataPoint(mb, networkMetrics.IO, s, currentTime)
	recordNetworkDataPoint(mb, networkMetrics.Errors, s, currentTime)
}

func recordNetworkDataPoint(mb *metadata.MetricsBuilder, recordDataPoint metadata.RecordIntDataPointWithDirectionFunc, s *stats.NetworkStats, currentTime pcommon.Timestamp) {
	if s.RxBytes == nil && s.TxBytes == nil {
		return
	}

	recordDataPoint(mb, currentTime, int64(*s.RxBytes), s.Name, metadata.AttributeDirectionReceive)
	recordDataPoint(mb, currentTime, int64(*s.TxBytes), s.Name, metadata.AttributeDirectionTransmit)
}
