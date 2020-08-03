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

func memMetrics(prefix string, s *MemoryStats) []*metricspb.Metric {
	return applyCurrentTime([]*metricspb.Metric{
		memUsageMetric(prefix, s),
		memMaxUsageMetric(prefix, s),
		memLimitMetric(prefix, s),
	}, time.Now())
}

func memUsageMetric(prefix string, s *MemoryStats) *metricspb.Metric {
	return intGauge(prefix+"memory.usage", "Bytes", s.Usage)
}

func memMaxUsageMetric(prefix string, s *MemoryStats) *metricspb.Metric {
	return intGauge(prefix+"memory.maxusage", "Bytes", s.MaxUsage)
}

func memLimitMetric(prefix string, s *MemoryStats) *metricspb.Metric {
	return intGauge(prefix+"memory.limit", "Bytes", s.Limit)
}
