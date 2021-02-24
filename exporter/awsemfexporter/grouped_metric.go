// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package awsemfexporter

import (
	"strconv"

	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"
)

// GroupedMetric defines set of metrics with same namespace, timestamp and labels
type GroupedMetric struct {
	Labels   map[string]string
	Metrics  map[string]*MetricInfo
	Metadata CWMetricMetadata
}

// MetricInfo defines value and unit for OT Metrics
type MetricInfo struct {
	Value interface{}
	Unit  string
}

// addToGroupedMetric processes OT metrics and adds them into GroupedMetric buckets
func addToGroupedMetric(pmd *pdata.Metric, groupedMetrics map[interface{}]*GroupedMetric, metadata CWMetricMetadata, logger *zap.Logger, descriptor map[string]MetricDescriptor) {
	if pmd == nil {
		return
	}

	metricName := pmd.Name()
	dps := getDataPoints(pmd, metadata, logger)
	if dps == nil || dps.Len() == 0 {
		return
	}

	for i := 0; i < dps.Len(); i++ {
		dp := dps.At(i)
		labels := dp.Labels
		metric := &MetricInfo{
			Value: dp.Value,
			Unit:  translateUnit(pmd, descriptor),
		}

		if dp.TimestampMs > 0 {
			metadata.TimestampMs = dp.TimestampMs
		}

		// Extra params to use when grouping metrics
		groupKey := rateKeyParams{
			namespaceKey: metadata.Namespace,
			timestampKey: strconv.FormatInt(metadata.TimestampMs, 10),
			logGroupKey:  metadata.LogGroup,
			logStreamKey: metadata.LogStream,
		}

		sortedLabels := getSortedLabels(labels)
		groupKey.labels = sortedLabels
		if _, ok := groupedMetrics[groupKey]; ok {
			// if metricName already exists in metrics map, print warning log
			if _, ok := groupedMetrics[groupKey].Metrics[metricName]; ok {
				logger.Warn(
					"Duplicate metric found",
					zap.String("Name", metricName),
					zap.Any("Labels", labels),
				)
			} else {
				groupedMetrics[groupKey].Metrics[metricName] = metric
			}
		} else {
			groupedMetrics[groupKey] = &GroupedMetric{
				Labels:   labels,
				Metrics:  map[string]*MetricInfo{(metricName): metric},
				Metadata: metadata,
			}
		}
	}
}

func translateUnit(metric *pdata.Metric, descriptor map[string]MetricDescriptor) string {
	unit := metric.Unit()
	if descriptor, exists := descriptor[metric.Name()]; exists {
		if unit == "" || descriptor.overwrite {
			return descriptor.unit
		}
	}
	switch unit {
	case "ms":
		unit = "Milliseconds"
	case "s":
		unit = "Seconds"
	case "us":
		unit = "Microseconds"
	case "By":
		unit = "Bytes"
	case "Bi":
		unit = "Bits"
	}
	return unit
}
