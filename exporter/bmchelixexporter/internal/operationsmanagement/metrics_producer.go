// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package operationsmanagement // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/bmchelixexporter/internal/operationsmanagement"

import (
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	conventions "go.opentelemetry.io/otel/semconv/v1.27.0"
	"go.uber.org/zap"
)

// BMCHelixOMMetric represents the structure of the payload that will be sent to BMC Helix Operations Management
type BMCHelixOMMetric struct {
	Labels  map[string]string  `json:"labels"`
	Samples []BMCHelixOMSample `json:"samples"`
}

// BMCHelixOMSample represents the individual sample for a metric
type BMCHelixOMSample struct {
	Value     float64 `json:"value"`
	Timestamp int64   `json:"timestamp"`
}

// MetricsProducer is responsible for converting OpenTelemetry metrics into BMC Helix Operations Management metrics
type MetricsProducer struct {
	logger *zap.Logger
}

// NewMetricsProducer creates a new MetricsProducer
func NewMetricsProducer(logger *zap.Logger) *MetricsProducer {
	return &MetricsProducer{
		logger: logger,
	}
}

// ProduceHelixPayload takes the OpenTelemetry metrics and converts them into the BMC Helix Operations Management metric format
func (mp *MetricsProducer) ProduceHelixPayload(metrics pmetric.Metrics) ([]BMCHelixOMMetric, error) {
	helixMetrics := []BMCHelixOMMetric{}
	containerParentEntities := map[string]BMCHelixOMMetric{}

	// Iterate through each pmetric.ResourceMetrics instance
	rmetrics := metrics.ResourceMetrics()
	for i := 0; i < rmetrics.Len(); i++ {
		resourceMetric := rmetrics.At(i)
		resource := resourceMetric.Resource()

		// Extract resource-level attributes (e.g., "host.name", "service.instance.id")
		resourceAttrs := extractResourceAttributes(resource)

		// Iterate through each pmetric.ScopeMetrics within the pmetric.ResourceMetrics instance
		scopeMetrics := resourceMetric.ScopeMetrics()
		for j := 0; j < scopeMetrics.Len(); j++ {
			scopeMetric := scopeMetrics.At(j)

			// Iterate through each individual pmetric.Metric instance
			metrics := scopeMetric.Metrics()
			for k := 0; k < metrics.Len(); k++ {
				metric := metrics.At(k)

				// Create the payload for each metric
				newMetrics, err := mp.createHelixMetrics(metric, resourceAttrs)
				if err != nil {
					mp.logger.Warn("Failed to create Helix metrics", zap.Error(err))
					continue
				}

				// Loop through the newly created metrics and append them to the helixMetrics slice
				// while also creating parent entities for container metrics
				for _, m := range newMetrics {
					helixMetrics = appendMetricWithParentEntity(helixMetrics, m, containerParentEntities)
				}
			}
		}
	}
	return helixMetrics, nil
}

// appends the metric to the helixMetrics slice and creates a parent entity if it doesn't exist
func appendMetricWithParentEntity(helixMetrics []BMCHelixOMMetric, helixMetric BMCHelixOMMetric, containerParentEntities map[string]BMCHelixOMMetric) []BMCHelixOMMetric {
	// Extract parent entity information
	parentEntityTypeID := fmt.Sprintf("%s_container", helixMetric.Labels["entityTypeId"])
	parentEntityID := fmt.Sprintf("%s:%s:%s:%s", helixMetric.Labels["source"], helixMetric.Labels["hostname"], parentEntityTypeID, parentEntityTypeID)

	// Create a parent entity if not already created
	if _, exists := containerParentEntities[parentEntityID]; !exists {
		parentMetric := BMCHelixOMMetric{
			Labels: map[string]string{
				"entityId":               parentEntityID,
				"entityName":             parentEntityTypeID,
				"entityTypeId":           parentEntityTypeID,
				"hostname":               helixMetric.Labels["hostname"],
				"source":                 helixMetric.Labels["source"],
				"isDeviceMappingEnabled": helixMetric.Labels["isDeviceMappingEnabled"],
				"hostType":               helixMetric.Labels["hostType"],
				"metricName":             "identity", // Represents the parent entity itself
			},
			Samples: []BMCHelixOMSample{}, // Parent entities don't have samples
		}
		containerParentEntities[parentEntityID] = parentMetric
		helixMetrics = append(helixMetrics, parentMetric)
	}

	// Add parent reference to the child metric
	helixMetric.Labels["parentEntityName"] = parentEntityTypeID
	helixMetric.Labels["parentEntityTypeId"] = parentEntityTypeID

	return append(helixMetrics, helixMetric)
}

// createHelixMetrics converts each OpenTelemetry datapoint into an individual BMCHelixOMMetric
func (mp *MetricsProducer) createHelixMetrics(metric pmetric.Metric, resourceAttrs map[string]string) ([]BMCHelixOMMetric, error) {
	var helixMetrics []BMCHelixOMMetric

	switch metric.Type() {
	case pmetric.MetricTypeSum:
		for i := 0; i < metric.Sum().DataPoints().Len(); i++ {
			dp := metric.Sum().DataPoints().At(i)
			metricPayload, err := mp.createSingleDatapointMetric(dp, metric, resourceAttrs)
			if err != nil {
				mp.logger.Warn("Failed to create Helix metric from datapoint", zap.Error(err))
				continue
			}
			helixMetrics = append(helixMetrics, *metricPayload)
		}
	case pmetric.MetricTypeGauge:
		for i := 0; i < metric.Gauge().DataPoints().Len(); i++ {
			dp := metric.Gauge().DataPoints().At(i)
			metricPayload, err := mp.createSingleDatapointMetric(dp, metric, resourceAttrs)
			if err != nil {
				mp.logger.Warn("Failed to create Helix metric from datapoint", zap.Error(err))
				continue
			}
			helixMetrics = append(helixMetrics, *metricPayload)
		}
	default:
		return nil, fmt.Errorf("unsupported metric type %s", metric.Type())
	}

	return helixMetrics, nil
}

// createSingleDatapointMetric creates a single BMCHelixOMMetric from a single OpenTelemetry datapoint
func (mp *MetricsProducer) createSingleDatapointMetric(dp pmetric.NumberDataPoint, metric pmetric.Metric, resourceAttrs map[string]string) (*BMCHelixOMMetric, error) {
	labels := make(map[string]string)
	labels["source"] = "OTEL"

	// Add resource attributes
	for k, v := range resourceAttrs {
		labels[k] = v
	}

	// Set the metric unit
	labels["unit"] = metric.Unit()

	// Set the host type
	labels["hostType"] = "server"

	// Indicates the monitor in the hierarchy that is mapped to the device
	labels["isDeviceMappingEnabled"] = "true"

	// Update the metric name for the BMC Helix Operations Management payload
	labels["metricName"] = metric.Name()

	// Update the entity information
	err := mp.updateEntityInformation(labels, metric.Name(), resourceAttrs, dp.Attributes().AsRaw())
	if err != nil {
		return nil, err
	}

	sample := newSample(dp)

	return &BMCHelixOMMetric{
		Labels:  labels,
		Samples: []BMCHelixOMSample{sample},
	}, nil
}

// Update the entity information for the BMC Helix Operations Management payload
func (mp *MetricsProducer) updateEntityInformation(labels map[string]string, metricName string, resourceAttrs map[string]string, dpAttributes map[string]any) error {
	// Try to get the hostname from resource attributes first
	hostname, found := resourceAttrs[string(conventions.HostNameKey)]
	if !found || hostname == "" {
		// Fallback to metric attributes if not found or empty in resource attributes
		maybeHostname, ok := dpAttributes[string(conventions.HostNameKey)].(string)
		if !ok || maybeHostname == "" {
			return fmt.Errorf("the hostname is required for the BMC Helix Operations Management payload but not set for metric %s. Metric datapoint will be skipped", metricName)
		}
		hostname = maybeHostname
	}

	// Add the hostname as a label (required for BMC Helix Operations Management payload)
	labels["hostname"] = hostname

	// Convert metricAttrs from map[string]any to map[string]string for compatibility
	stringMetricAttrs := make(map[string]string)
	for k, v := range dpAttributes {
		stringMetricAttrs[k] = fmt.Sprintf("%v", v)
		labels[k] = fmt.Sprintf("%v", v)
	}

	// Add the resource attributes to the metric attributes
	for k, v := range resourceAttrs {
		stringMetricAttrs[k] = v
	}

	// entityTypeId is required for the BMC Helix Operations Management payload
	entityTypeID := stringMetricAttrs["entityTypeId"]
	if entityTypeID == "" {
		return fmt.Errorf("the entityTypeId is required for the BMC Helix Operations Management payload but not set for metric %s. Metric datapoint will be skipped", metricName)
	}

	// entityName is required for the BMC Helix Operations Management payload
	entityName := stringMetricAttrs["entityName"]
	if entityName == "" {
		return fmt.Errorf("the entityName is required for the BMC Helix Operations Management payload but not set for metric %s. Metric datapoint will be skipped", metricName)
	}

	instanceName := stringMetricAttrs["instanceName"]
	if instanceName == "" {
		instanceName = entityName
	}

	// Set the entityTypeId, entityId, instanceName and entityName in labels
	labels["entityTypeId"] = entityTypeID
	labels["entityName"] = entityName
	labels["instanceName"] = instanceName
	labels["entityId"] = fmt.Sprintf("%s:%s:%s:%s", labels["source"], labels["hostname"], entityTypeID, entityName)
	return nil
}

// newSample creates a new BMCHelixOMSample from the OpenTelemetry data point
func newSample(dp pmetric.NumberDataPoint) BMCHelixOMSample {
	var value float64
	switch dp.ValueType() {
	case pmetric.NumberDataPointValueTypeDouble:
		value = dp.DoubleValue()
	case pmetric.NumberDataPointValueTypeInt:
		value = float64(dp.IntValue()) // convert int to float for consistency
	}

	return BMCHelixOMSample{
		Value:     value,
		Timestamp: dp.Timestamp().AsTime().Unix() * 1000,
	}
}

// extractResourceAttributes extracts the resource attributes from OpenTelemetry resource data
func extractResourceAttributes(resource pcommon.Resource) map[string]string {
	attributes := make(map[string]string)

	for k, v := range resource.Attributes().All() {
		attributes[k] = v.AsString()
	}

	return attributes
}
