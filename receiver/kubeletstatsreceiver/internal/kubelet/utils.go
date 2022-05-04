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
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver/internal/metadata"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func recordIntDataPoint(mb *metadata.MetricsBuilder, recordDataPoint metadata.RecordIntDataPointFunc, value *uint64, currentTime pcommon.Timestamp) {
	if value == nil {
		return
	}
	recordDataPoint(mb, currentTime, int64(*value))
}

//func fillDoubleGauge(dest pmetric.Metric, metricInt metadata.MetricIntf, value float64, currentTime pcommon.Timestamp) {
//	metricInt.Init(dest)
//	dp := dest.Gauge().DataPoints().AppendEmpty()
//	dp.SetDoubleVal(value)
//	dp.SetTimestamp(currentTime)
//}
//
//func addIntGauge(dest pmetric.MetricSlice, metricInt metadata.MetricIntf, value *uint64, currentTime pcommon.Timestamp) {
//	if value == nil {
//		return
//	}
//	fillIntGauge(dest.AppendEmpty(), metricInt, int64(*value), currentTime)
//}
//
//func fillIntGauge(dest pmetric.Metric, metricInt metadata.MetricIntf, value int64, currentTime pcommon.Timestamp) {
//	metricInt.Init(dest)
//	dp := dest.Gauge().DataPoints().AppendEmpty()
//	dp.SetIntVal(value)
//	dp.SetTimestamp(currentTime)
//}
//
//func fillDoubleSum(dest pmetric.Metric, metricInt metadata.MetricIntf, value float64, startTime pcommon.Timestamp, currentTime pcommon.Timestamp) {
//	metricInt.Init(dest)
//	dp := dest.Sum().DataPoints().AppendEmpty()
//	dp.SetDoubleVal(value)
//	dp.SetStartTimestamp(startTime)
//	dp.SetTimestamp(currentTime)
//}
