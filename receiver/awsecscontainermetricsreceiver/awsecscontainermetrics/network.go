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

func networkMetrics(prefix string, stats map[string]NetworkStats, labelKeys []*metricspb.LabelKey, labelValues []*metricspb.LabelValue) []*metricspb.Metric {

	netStatArray := getNetworkStats(stats)
	return applyCurrentTime([]*metricspb.Metric{
		intGauge(prefix+"network.rx_bytes", "Bytes", &netStatArray[0], labelKeys, labelValues),
		intGauge(prefix+"network.rx_packets", "Bytes", &netStatArray[1], labelKeys, labelValues),
		intGauge(prefix+"network.rx_errors", "Bytes", &netStatArray[2], labelKeys, labelValues),
		intGauge(prefix+"network.rx_dropped", "Bytes", &netStatArray[3], labelKeys, labelValues),
		intGauge(prefix+"network.tx_bytes", "Bytes", &netStatArray[4], labelKeys, labelValues),
		intGauge(prefix+"network.tx_packets", "Bytes", &netStatArray[5], labelKeys, labelValues),
		intGauge(prefix+"network.tx_errors", "Bytes", &netStatArray[6], labelKeys, labelValues),
		intGauge(prefix+"network.tx_dropped", "Bytes", &netStatArray[7], labelKeys, labelValues),
	}, time.Now())
}

func getNetworkStats(stats map[string]NetworkStats) [8]uint64 {
	var netStatArray [8]uint64
	for _, netStat := range stats {
		netStatArray[0] += *netStat.RxBytes
		netStatArray[1] += *netStat.RxPackets
		netStatArray[2] += *netStat.RxErrors
		netStatArray[3] += *netStat.RxDropped

		netStatArray[4] += *netStat.TxBytes
		netStatArray[5] += *netStat.TxPackets
		netStatArray[6] += *netStat.TxErrors
		netStatArray[7] += *netStat.TxDropped
	}
	return netStatArray
}
