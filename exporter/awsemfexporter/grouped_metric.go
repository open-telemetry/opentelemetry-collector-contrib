// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsemfexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsemfexporter"

import (
	"encoding/json"
	"math"
	"strings"

	"go.opentelemetry.io/collector/pdata/pmetric"
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
	value any
	unit  string
}

// addToGroupedMetric processes OT metrics and adds them into GroupedMetric buckets
func addToGroupedMetric(
	pmd pmetric.Metric,
	groupedMetrics map[any]*groupedMetric,
	metadata cWMetricMetadata,
	patternReplaceSucceeded bool,
	descriptor map[string]MetricDescriptor,
	config *Config,
	calculators *emfCalculators,
) error {
	dps := getDataPoints(pmd, metadata, config.logger)
	if dps == nil || dps.Len() == 0 {
		return nil
	}

	filteredDps := filterAndCalculateDps(dps, pmd.Name(), metadata, config, calculators)

	if shouldConvertToDistribution(pmd, config) {
		histogram, labels, updatedMetadata := convertToDistribution(filteredDps, metadata, patternReplaceSucceeded, config)
		if histogram != nil {
			upsertGroupedMetric(groupedMetrics, updatedMetadata, labels, pmd.Name(), histogram, translateUnit(pmd, descriptor), config.logger)
		}
	} else {
		for i, dp := range filteredDps {
			labels := enrichLabels(dp, config)
			metadata = replacePatternsIfNeeded(metadata, labels, config, patternReplaceSucceeded)
			if dp.timestampMs > 0 {
				metadata.timestampMs = dp.timestampMs
			}
			// Extra params to use when grouping metrics
			if metadata.metricDataType != pmetric.MetricTypeSummary || !config.DetailedMetrics {
				// Summary metrics can be split into separate datapoints when using DetailedMetrics, but we still want to group
				// them together into one EMF log event, so don't set batchIndex when it's a summary metric
				metadata.batchIndex = i
			}

			// Handle metric types with container insights
			if metadata.receiver == containerInsightsReceiver {
				// For container insights, treat gauge metrics as sum to keep metrics in the same EMF record
				if metadata.metricDataType == pmetric.MetricTypeGauge {
					metadata.metricDataType = pmetric.MetricTypeSum
				}
			}
			upsertGroupedMetric(groupedMetrics, metadata, labels, dp.name, dp.value, translateUnit(pmd, descriptor), config.logger)
		}
	}

	return nil
}

type kubernetesObj struct {
	ContainerName string                `json:"container_name,omitempty"`
	Docker        *internalDockerObj    `json:"docker,omitempty"`
	Host          string                `json:"host,omitempty"`
	Labels        *internalLabelsObj    `json:"labels,omitempty"`
	NamespaceName string                `json:"namespace_name,omitempty"`
	PodID         string                `json:"pod_id,omitempty"`
	PodName       string                `json:"pod_name,omitempty"`
	PodOwners     *internalPodOwnersObj `json:"pod_owners,omitempty"`
	ServiceName   string                `json:"service_name,omitempty"`
}

type internalDockerObj struct {
	ContainerID string `json:"container_id,omitempty"`
}

type internalLabelsObj struct {
	App             string `json:"app,omitempty"`
	PodTemplateHash string `json:"pod-template-hash,omitempty"`
}

type internalPodOwnersObj struct {
	OwnerKind string `json:"owner_kind,omitempty"`
	OwnerName string `json:"owner_name,omitempty"`
}

func addKubernetesWrapper(labels map[string]string) {
	// fill in obj
	filledInObj := kubernetesObj{
		ContainerName: mapGetHelper(labels, "container"),
		Docker: &internalDockerObj{
			ContainerID: mapGetHelper(labels, "container_id"),
		},
		Host: mapGetHelper(labels, "NodeName"),
		Labels: &internalLabelsObj{
			App:             mapGetHelper(labels, "app"),
			PodTemplateHash: mapGetHelper(labels, "pod-template-hash"),
		},
		NamespaceName: mapGetHelper(labels, "Namespace"),
		PodID:         mapGetHelper(labels, "PodId"),
		PodName:       mapGetHelper(labels, "PodName"),
		PodOwners: &internalPodOwnersObj{
			OwnerKind: mapGetHelper(labels, "owner_kind"),
			OwnerName: mapGetHelper(labels, "owner_name"),
		},
		ServiceName: mapGetHelper(labels, "Service"),
	}

	// handle nested empty object
	if filledInObj.Docker.ContainerID == "" {
		filledInObj.Docker = nil
	}

	if filledInObj.Labels.App == "" && filledInObj.Labels.PodTemplateHash == "" {
		filledInObj.Labels = nil
	}

	if filledInObj.PodOwners.OwnerKind == "" && filledInObj.PodOwners.OwnerName == "" {
		filledInObj.PodOwners = nil
	}

	jsonBytes, _ := json.Marshal(filledInObj)
	labels["kubernetes"] = string(jsonBytes)
}

func mapGetHelper(labels map[string]string, key string) string {
	val, ok := labels[key]
	if ok {
		return val
	}

	return ""
}

func translateUnit(metric pmetric.Metric, descriptor map[string]MetricDescriptor) string {
	unit := metric.Unit()
	if descriptor, exists := descriptor[metric.Name()]; exists {
		if unit == "" || descriptor.Overwrite {
			return descriptor.Unit
		}
	}
	switch unit {
	case "1":
		unit = ""
	case "ns":
		// CloudWatch doesn't support Nanoseconds
		unit = ""
	case "ms":
		unit = "Milliseconds"
	case "s":
		unit = "Seconds"
	case "us":
		unit = "Microseconds"
	case "By":
		unit = "Bytes"
	case "bit":
		unit = "Bits"
	}
	return unit
}

func shouldConvertToDistribution(pmd pmetric.Metric, config *Config) bool {
	if pmd.Type() != pmetric.MetricTypeGauge {
		return false
	}
	// Check if the current metric is in the MetricAsDistribution list
	for _, name := range config.MetricAsDistribution {
		if name == pmd.Name() {
			return true
		}
	}
	return false
}

func filterAndCalculateDps(dps dataPoints, metricName string, metadata cWMetricMetadata, config *Config, calculators *emfCalculators) []dataPoint {
	var result []dataPoint
	for i := 0; i < dps.Len(); i++ {
		// Drop stale or NaN metric values
		if isStale, attrs := dps.IsStaleNaNInf(i); isStale {
			if config != nil && config.logger != nil {
				config.logger.Debug("dropped metric with nan value",
					zap.String("metric.name", metricName),
					zap.Any("metric.attributes", attrs))
			}
			continue
		}
		calculated, retained := dps.CalculateDeltaDatapoints(i, metadata.instrumentationScopeName, config.DetailedMetrics, calculators)
		if retained {
			result = append(result, calculated...)
		}
	}
	return result
}

func enrichLabels(dp dataPoint, config *Config) map[string]string {
	labels := dp.labels
	if metricType, ok := labels["Type"]; ok {
		if (metricType == "Pod" || metricType == "Container") && config.EKSFargateContainerInsightsEnabled {
			addKubernetesWrapper(labels)
		}
	}
	return labels
}

func replacePatternsIfNeeded(metadata cWMetricMetadata, labels map[string]string, config *Config, patternReplaceSucceeded bool) cWMetricMetadata {
	// if patterns were found in config file and weren't replaced by resource attributes, replace those patterns with metric labels.
	// if patterns are provided for a valid key and that key doesn't exist in the resource attributes, it is replaced with `undefined`.
	if !patternReplaceSucceeded {
		if strings.Contains(metadata.logGroup, "undefined") {
			metadata.logGroup, _ = replacePatterns(config.LogGroupName, labels, config.logger)
		}
		if strings.Contains(metadata.logStream, "undefined") {
			metadata.logStream, _ = replacePatterns(config.LogStreamName, labels, config.logger)
		}
	}
	return metadata
}

func upsertGroupedMetric(
	groupedMetrics map[any]*groupedMetric,
	metadata cWMetricMetadata,
	labels map[string]string,
	metricName string,
	metricVal any,
	unit string,
	logger *zap.Logger,
) {
	metric := &metricInfo{value: metricVal, unit: unit}
	groupKey := aws.NewKey(metadata.groupedMetricMetadata, labels)

	if _, ok := groupedMetrics[groupKey]; ok {
		// if MetricName already exists in metrics map, print warning log
		if _, ok := groupedMetrics[groupKey].metrics[metricName]; ok {
			logger.Warn("Duplicate metric found", zap.String("Name", metricName), zap.Any("Labels", labels))
		} else {
			groupedMetrics[groupKey].metrics[metricName] = metric
		}
	} else {
		groupedMetrics[groupKey] = &groupedMetric{
			labels:   labels,
			metrics:  map[string]*metricInfo{metricName: metric},
			metadata: metadata,
		}
	}
}

// convertToDistribution converts a collection of gauge data points into a distribution representation, e.g. values and counts.
func convertToDistribution(
	dps []dataPoint,
	metadata cWMetricMetadata,
	patternReplaceSucceeded bool,
	config *Config,
) (histogram *cWMetricHistogram, labels map[string]string, updatedMetadata cWMetricMetadata) {
	var values []float64
	var timestampMs int64

	// Extract float values from data points and find the latest timestamp
	for _, dp := range dps {
		if dp.timestampMs > timestampMs {
			timestampMs = dp.timestampMs
			labels = enrichLabels(dp, config)
		}
		if v, ok := dp.value.(float64); ok {
			values = append(values, v)
		}
	}

	if len(values) == 0 {
		return nil, nil, metadata
	}

	updatedMetadata = replacePatternsIfNeeded(metadata, labels, config, patternReplaceSucceeded)
	updatedMetadata.metricDataType = pmetric.MetricTypeGauge
	updatedMetadata.timestampMs = timestampMs

	histogram = &cWMetricHistogram{
		Values: []float64{},
		Counts: []float64{},
		Count:  uint64(len(values)),
		Sum:    0,
		Min:    math.MaxFloat64,
		Max:    -math.MaxFloat64,
	}

	// Calculate sum, min, max and count frequencies for each unique value
	countMap := make(map[float64]float64)
	for _, v := range values {
		histogram.Sum += v
		if v < histogram.Min {
			histogram.Min = v
		}
		if v > histogram.Max {
			histogram.Max = v
		}
		countMap[v]++
	}

	// Pre-allocate slices to avoid multiple allocations during append
	histogram.Values = make([]float64, 0, len(countMap))
	histogram.Counts = make([]float64, 0, len(countMap))
	for val, cnt := range countMap {
		histogram.Values = append(histogram.Values, val)
		histogram.Counts = append(histogram.Counts, cnt)
	}

	return histogram, labels, updatedMetadata
}
