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

func networkMetrics(prefix string, stats map[string]NetworkStats) []*metricspb.Metric {
	var rBytes, rPackets, rErrors, rDropped uint64
	var tBytes, tPackets, tErrors, tDropped uint64
	for _, netStat := range stats {
		rBytes += *netStat.RxBytes
		rPackets += *netStat.RxPackets
		rErrors += *netStat.RxErrors
		rDropped += *netStat.RxDropped

		tBytes += *netStat.TxBytes
		tPackets += *netStat.TxPackets
		tErrors += *netStat.TxErrors
		tDropped += *netStat.TxDropped
	}
	return applyCurrentTime([]*metricspb.Metric{
		rxBytes(prefix, &rBytes),
		rxPackets(prefix, &rPackets),
		rxErrors(prefix, &rErrors),
		rxDropped(prefix, &rDropped),
		txBytes(prefix, &tBytes),
		txPackets(prefix, &tPackets),
		txErrors(prefix, &tErrors),
		txDropped(prefix, &tDropped),
	}, time.Now())
}

func rxBytes(prefix string, value *uint64) *metricspb.Metric {
	return intGauge(prefix+"network.rx_bytes", "Bytes", value)
}

func rxPackets(prefix string, value *uint64) *metricspb.Metric {
	return intGauge(prefix+"network.rx_packets", "Bytes", value)
}

func rxErrors(prefix string, value *uint64) *metricspb.Metric {
	return intGauge(prefix+"network.rx_errors", "Bytes", value)
}

func rxDropped(prefix string, value *uint64) *metricspb.Metric {
	return intGauge(prefix+"network.rx_dropped", "Bytes", value)
}

func txBytes(prefix string, value *uint64) *metricspb.Metric {
	return intGauge(prefix+"network.tx_bytes", "Bytes", value)
}

func txPackets(prefix string, value *uint64) *metricspb.Metric {
	return intGauge(prefix+"network.tx_packets", "Bytes", value)
}

func txErrors(prefix string, value *uint64) *metricspb.Metric {
	return intGauge(prefix+"network.tx_errors", "Bytes", value)
}

func txDropped(prefix string, value *uint64) *metricspb.Metric {
	return intGauge(prefix+"network.tx_dropped", "Bytes", value)
}
