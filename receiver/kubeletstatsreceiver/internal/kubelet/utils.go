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

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/metadata"
)

func fillDoubleGauge(dest pdata.Metric, prefix string, metricInt metadata.MetricIntf, value float64, currentTime pdata.Timestamp) {
	metricInt.Init(dest)
	dest.SetName(prefix + dest.Name())
	dp := dest.Gauge().DataPoints().AppendEmpty()
	dp.SetDoubleVal(value)
	dp.SetTimestamp(currentTime)
}

func addIntGauge(dest pdata.MetricSlice, prefix string, metricInt metadata.MetricIntf, value *uint64, currentTime pdata.Timestamp) {
	if value == nil {
		return
	}
	fillIntGauge(dest.AppendEmpty(), prefix, metricInt, int64(*value), currentTime)
}

func fillIntGauge(dest pdata.Metric, prefix string, metricInt metadata.MetricIntf, value int64, currentTime pdata.Timestamp) {
	metricInt.Init(dest)
	dest.SetName(prefix + dest.Name())
	dp := dest.Gauge().DataPoints().AppendEmpty()
	dp.SetIntVal(value)
	dp.SetTimestamp(currentTime)
}

func fillDoubleSum(dest pdata.Metric, prefix string, metricInt metadata.MetricIntf, value float64, startTime pdata.Timestamp, currentTime pdata.Timestamp) {
	metricInt.Init(dest)
	dest.SetName(prefix + dest.Name())
	dp := dest.Sum().DataPoints().AppendEmpty()
	dp.SetDoubleVal(value)
	dp.SetStartTimestamp(startTime)
	dp.SetTimestamp(currentTime)
}
