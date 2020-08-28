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

package awsecscontainermetrics

import (
	"time"

	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
)

func networkRateMetrics(prefix string, stats *NetworkRateStats, labelKeys []*metricspb.LabelKey, labelValues []*metricspb.LabelValue) []*metricspb.Metric {
	return applyCurrentTime([]*metricspb.Metric{
		doubleGauge(prefix+"network_rate.rx_bytes_per_sec", "Bytes/Sec", stats.RxBytesPerSecond, labelKeys, labelValues),
		doubleGauge(prefix+"network_rate.tx_bytes_per_sec", "Bytes/Sec", stats.TxBytesPerSecond, labelKeys, labelValues),
	}, time.Now())
}

// func rxBytesPerSecond(prefix string, value *float64) *metricspb.Metric {
// 	return doubleGauge(prefix+"network_rate.rx_bytes_per_sec", "Bytes/Sec", value)
// }

// func txBytesPerSecond(prefix string, value *float64) *metricspb.Metric {
// 	return doubleGauge(prefix+"network_rate.tx_bytes_per_sec", "Bytes/Sec", value)
// }
