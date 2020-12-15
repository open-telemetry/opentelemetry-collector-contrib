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

const (
	namespaceKey  = "CloudWatchNamespace"
	timestampKey  = "CloudWatchTimestamp"
	metricNameKey = "CloudWatchMetricName"
	logGroupKey   = "CloudWatchLogGroup"
	logStreamKey  = "CloudWatchLogStream"
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
func addToGroupedMetric(pmd *pdata.Metric, groupedMetrics map[string]*GroupedMetric, metadata CWMetricMetadata, logger *zap.Logger) {
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
			Unit:  pmd.Unit(),
		}

		if dp.Timestamp > 0 {
			metadata.Timestamp = dp.Timestamp
		}

		// Extra params to use when grouping metrics
		groupKeyParams := map[string]string{
			(namespaceKey): metadata.Namespace,
			(timestampKey): strconv.FormatInt(metadata.Timestamp, 10),
			(logGroupKey):  metadata.LogGroup,
			(logStreamKey): metadata.LogStream,
		}

		groupKey := createMetricKey(labels, groupKeyParams)
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
