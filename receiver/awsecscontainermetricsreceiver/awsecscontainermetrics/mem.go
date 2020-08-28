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

const (
	// BytesInMiB is the number of bytes in a MebiByte.
	BytesInMiB = 1024 * 1024
)

func memMetrics(prefix string, stats *MemoryStats, containerMetadata ContainerMetadata, labelKeys []*metricspb.LabelKey, labelValues []*metricspb.LabelValue) []*metricspb.Metric {
	memoryUtilizedInMb := (*stats.Usage - stats.Stats["cache"]) / BytesInMiB
	stats.MemoryUtilized = &memoryUtilizedInMb
	return applyCurrentTime([]*metricspb.Metric{
		intGauge(prefix+"memory.usage", "Bytes", stats.Usage, labelKeys, labelValues),
		intGauge(prefix+"memory.maxusage", "MB", stats.MaxUsage, labelKeys, labelValues),
		intGauge(prefix+"memory.limit", "MB", stats.Limit, labelKeys, labelValues),
		intGauge(prefix+"memory.utilized", "MB", stats.MemoryUtilized, labelKeys, labelValues),
		intGauge(prefix+"memory.reserved", "MB", containerMetadata.Limits.Memory, labelKeys, labelValues),
	}, time.Now())
}

// func memUsageMetric(prefix string, value *uint64, labelKeys []*metricspb.LabelKey, labelValues []*metricspb.LabelValue) *metricspb.Metric {
// 	return intGauge(prefix+"memory.usage", "Bytes", value, labelKeys, labelValues)
// }

// func memMaxUsageMetric(prefix string, value *uint64, labelKeys []*metricspb.LabelKey, labelValues []*metricspb.LabelValue) *metricspb.Metric {
// 	return intGauge(prefix+"memory.maxusage", "MB", value, labelKeys, labelValues)
// }

// func memLimitMetric(prefix string, value *uint64, labelKeys []*metricspb.LabelKey, labelValues []*metricspb.LabelValue) *metricspb.Metric {
// 	return intGauge(prefix+"memory.limit", "MB", value, labelKeys, labelValues)
// }

// func memUtilizedMetric(prefix string, value *uint64, labelKeys []*metricspb.LabelKey, labelValues []*metricspb.LabelValue) *metricspb.Metric {
// 	return intGauge(prefix+"memory.utilized", "MB", value, labelKeys, labelValues)
// }

// func memReserved(prefix string, value *uint64, labelKeys []*metricspb.LabelKey, labelValues []*metricspb.LabelValue) *metricspb.Metric {
// 	return intGauge(prefix+"memory.reserved", "MB", value, labelKeys, labelValues)
// }
