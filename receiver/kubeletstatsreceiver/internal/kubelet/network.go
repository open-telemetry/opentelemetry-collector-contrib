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
	"go.opentelemetry.io/collector/pdata/pmetric"
	stats "k8s.io/kubelet/pkg/apis/stats/v1alpha1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/metadata"
)

func addNetworkMetrics(dest pmetric.MetricSlice, ioMetricInt metadata.MetricIntf, errorsMetricInt metadata.MetricIntf, s *stats.NetworkStats, startTime pcommon.Timestamp, currentTime pcommon.Timestamp) {
	if s == nil {
		return
	}
	addNetworkIOMetric(dest, ioMetricInt, s, startTime, currentTime)
	addNetworkErrorsMetric(dest, errorsMetricInt, s, startTime, currentTime)
}

func addNetworkIOMetric(dest pmetric.MetricSlice, metricInt metadata.MetricIntf, s *stats.NetworkStats, startTime pcommon.Timestamp, currentTime pcommon.Timestamp) {
	if s.RxBytes == nil && s.TxBytes == nil {
		return
	}

	m := dest.AppendEmpty()
	metricInt.Init(m)

	fillNetworkDataPoint(m.Sum().DataPoints(), s.Name, metadata.AttributeDirection.Receive, s.RxBytes, startTime, currentTime)
	fillNetworkDataPoint(m.Sum().DataPoints(), s.Name, metadata.AttributeDirection.Transmit, s.TxBytes, startTime, currentTime)
}

func addNetworkErrorsMetric(dest pmetric.MetricSlice, metricInt metadata.MetricIntf, s *stats.NetworkStats, startTime pcommon.Timestamp, currentTime pcommon.Timestamp) {
	if s.RxBytes == nil && s.TxBytes == nil {
		return
	}

	m := dest.AppendEmpty()
	metricInt.Init(m)

	fillNetworkDataPoint(m.Sum().DataPoints(), s.Name, metadata.AttributeDirection.Receive, s.RxErrors, startTime, currentTime)
	fillNetworkDataPoint(m.Sum().DataPoints(), s.Name, metadata.AttributeDirection.Transmit, s.TxErrors, startTime, currentTime)
}

func fillNetworkDataPoint(dps pmetric.NumberDataPointSlice, interfaceName string, direction string, value *uint64, startTime pcommon.Timestamp, currentTime pcommon.Timestamp) {
	if value == nil {
		return
	}
	dp := dps.AppendEmpty()
	dp.Attributes().UpsertString(metadata.A.Interface, interfaceName)
	dp.Attributes().UpsertString(metadata.A.Direction, direction)
	dp.SetIntVal(int64(*value))
	dp.SetStartTimestamp(startTime)
	dp.SetTimestamp(currentTime)
}
