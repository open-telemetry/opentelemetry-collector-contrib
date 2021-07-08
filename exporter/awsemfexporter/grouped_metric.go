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
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"

	aws "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/metrics"
)

// groupedMetric defines set of metrics with same namespace, timestamp and labels
type groupedMetric struct {
	labels   map[string]string
	metrics  map[string]*metricInfo
	metadata cWMetricMetadata
}

// metricInfo defines value and unit for OT Metrics
type metricInfo struct {
	value interface{}
	unit  string
}

// addToGroupedMetric processes OT metrics and adds them into groupedMetric buckets
func addToGroupedMetric(pmd *pdata.Metric, groupedMetrics map[interface{}]*groupedMetric, metadata cWMetricMetadata, logger *zap.Logger, descriptor map[string]MetricDescriptor) {
	if pmd == nil {
		return
	}

	metricName := pmd.Name()
	dps := getDataPoints(pmd, metadata, logger)
	if dps == nil || dps.Len() == 0 {
		return
	}

	for i := 0; i < dps.Len(); i++ {
		dp, retained := dps.At(i)
		if !retained {
			continue
		}

		labels := dp.labels
		metric := &metricInfo{
			value: dp.value,
			unit:  translateUnit(pmd, descriptor),
		}

		if dp.timestampMs > 0 {
			metadata.timestampMs = dp.timestampMs
		}

		// Extra params to use when grouping metrics
		groupKey := groupedMetricKey(metadata.groupedMetricMetadata, labels)
		if _, ok := groupedMetrics[groupKey]; ok {
			// if metricName already exists in metrics map, print warning log
			if _, ok := groupedMetrics[groupKey].metrics[metricName]; ok {
				logger.Warn(
					"Duplicate metric found",
					zap.String("Name", metricName),
					zap.Any("Labels", labels),
				)
			} else {
				groupedMetrics[groupKey].metrics[metricName] = metric
			}
		} else {
			groupedMetrics[groupKey] = &groupedMetric{
				labels:   labels,
				metrics:  map[string]*metricInfo{(metricName): metric},
				metadata: metadata,
			}
		}
	}
}

func groupedMetricKey(metadata groupedMetricMetadata, labels map[string]string) aws.Key {
	return aws.NewKey(metadata, labels)
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
