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

func addNetworkMetrics(dest pdata.MetricSlice, prefix string, s *stats.NetworkStats, startTime pdata.Timestamp, currentTime pdata.Timestamp) {
	if s == nil {
		return
	}
	addNetworkIOMetric(dest, prefix, s, startTime, currentTime)
	addNetworkErrorsMetric(dest, prefix, s, startTime, currentTime)
}

func addNetworkIOMetric(dest pdata.MetricSlice, prefix string, s *stats.NetworkStats, startTime pdata.Timestamp, currentTime pdata.Timestamp) {
	if s.RxBytes == nil && s.TxBytes == nil {
		return
	}

	m := dest.AppendEmpty()
	metadata.M.NetworkIo.Init(m)
	m.SetName(prefix + m.Name())

	fillNetworkDataPoint(m.Sum().DataPoints(), s.Name, metadata.LabelDirection.Receive, s.RxBytes, startTime, currentTime)
	fillNetworkDataPoint(m.Sum().DataPoints(), s.Name, metadata.LabelDirection.Transmit, s.TxBytes, startTime, currentTime)
}

func addNetworkErrorsMetric(dest pdata.MetricSlice, prefix string, s *stats.NetworkStats, startTime pdata.Timestamp, currentTime pdata.Timestamp) {
	if s.RxBytes == nil && s.TxBytes == nil {
		return
	}

	m := dest.AppendEmpty()
	metadata.M.NetworkErrors.Init(m)
	m.SetName(prefix + m.Name())

	fillNetworkDataPoint(m.Sum().DataPoints(), s.Name, metadata.LabelDirection.Receive, s.RxErrors, startTime, currentTime)
	fillNetworkDataPoint(m.Sum().DataPoints(), s.Name, metadata.LabelDirection.Transmit, s.TxErrors, startTime, currentTime)
}

func fillNetworkDataPoint(dps pdata.NumberDataPointSlice, interfaceName string, direction string, value *uint64, startTime pdata.Timestamp, currentTime pdata.Timestamp) {
	if value == nil {
		return
	}
	dp := dps.AppendEmpty()
	dp.Attributes().UpsertString(metadata.L.Interface, interfaceName)
	dp.Attributes().UpsertString(metadata.L.Direction, direction)
	dp.SetIntVal(int64(*value))
	dp.SetStartTimestamp(startTime)
	dp.SetTimestamp(currentTime)
}
