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
	metricspb "github.com/census-instrumentation/opencensus-proto/gen-go/metrics/v1"
	stats "k8s.io/kubernetes/pkg/kubelet/apis/stats/v1alpha1"
)

func networkMetrics(prefix string, s *stats.NetworkStats) []*metricspb.Metric {
	if s == nil {
		return nil
	}
	// todo s.RxErrors s.TxErrors?
	return []*metricspb.Metric{
		rxBytesMetric(prefix, s),
		txBytesMetric(prefix, s),
		rxErrorsMetric(prefix, s),
		txErrorsMetric(prefix, s),
	}
}

const directionLabel = "direction"

func rxBytesMetric(prefix string, s *stats.NetworkStats) *metricspb.Metric {
	metric := cumulativeInt(prefix+"network.io", "By", s.RxBytes)
	applyLabels(metric, map[string]string{"interface": s.Name, directionLabel: "receive"})
	return metric
}

func txBytesMetric(prefix string, s *stats.NetworkStats) *metricspb.Metric {
	metric := cumulativeInt(prefix+"network.io", "By", s.TxBytes)
	applyLabels(metric, map[string]string{"interface": s.Name, directionLabel: "transmit"})
	return metric
}

func rxErrorsMetric(prefix string, s *stats.NetworkStats) *metricspb.Metric {
	metric := cumulativeInt(prefix+"network.errors", "By", s.RxErrors)
	applyLabels(metric, map[string]string{"interface": s.Name, directionLabel: "receive"})
	return metric
}

func txErrorsMetric(prefix string, s *stats.NetworkStats) *metricspb.Metric {
	metric := cumulativeInt(prefix+"network.errors", "By", s.TxErrors)
	applyLabels(metric, map[string]string{"interface": s.Name, directionLabel: "transmit"})
	return metric
}
