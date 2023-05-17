// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsemfexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsemfexporter"

import (
	"encoding/json"
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
	value interface{}
	unit  string
}

// addToGroupedMetric processes OT metrics and adds them into GroupedMetric buckets
func addToGroupedMetric(pmd pmetric.Metric, groupedMetrics map[interface{}]*groupedMetric, metadata cWMetricMetadata, patternReplaceSucceeded bool, logger *zap.Logger, descriptor map[string]MetricDescriptor, config *Config) error {

	dps := getDataPoints(pmd, metadata, logger)
	if dps == nil || dps.Len() == 0 {
		return nil
	}

	for i := 0; i < dps.Len(); i++ {
		dps, retained := dps.CalculateDeltaDatapoints(i, metadata.instrumentationScopeName, config.DetailedMetrics)
		if !retained {
			continue
		}

		for _, dp := range dps {
			labels := dp.labels

			if metricType, ok := labels["Type"]; ok {
				if (metricType == "Pod" || metricType == "Container") && config.EKSFargateContainerInsightsEnabled {
					addKubernetesWrapper(labels)
				}
			}

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

			metric := &metricInfo{
				value: dp.value,
				unit:  translateUnit(pmd, descriptor),
			}

			if dp.timestampMs > 0 {
				metadata.timestampMs = dp.timestampMs
			}

			// Extra params to use when grouping metrics
			groupKey := aws.NewKey(metadata.groupedMetricMetadata, labels)
			if _, ok := groupedMetrics[groupKey]; ok {
				// if MetricName already exists in metrics map, print warning log
				if _, ok := groupedMetrics[groupKey].metrics[dp.name]; ok {
					logger.Warn(
						"Duplicate metric found",
						zap.String("Name", dp.name),
						zap.Any("Labels", labels),
					)
				} else {
					groupedMetrics[groupKey].metrics[dp.name] = metric
				}
			} else {
				groupedMetrics[groupKey] = &groupedMetric{
					labels:   labels,
					metrics:  map[string]*metricInfo{(dp.name): metric},
					metadata: metadata,
				}
			}
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
