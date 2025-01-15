// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package bmchelixexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/bmchelixexporter"

import (
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	conventions "go.opentelemetry.io/collector/semconv/v1.27.0"
	"go.uber.org/zap"
)

// BmcHelixMetric represents the structure of the payload that will be sent to BMC Helix
type BmcHelixMetric struct {
	Labels  map[string]string `json:"labels"`
	Samples []BmcHelixSample  `json:"samples"`
}

// BmcHelixSample represents the individual sample for a metric
type BmcHelixSample struct {
	Value     float64 `json:"value"`
	Timestamp int64   `json:"timestamp"`
}

// MetricsProducer is responsible for converting OpenTelemetry metrics into BMC Helix metrics
type MetricsProducer struct {
	osHostname string
	logger     *zap.Logger
}

// newMetricsProducer creates a new MetricsProducer
func newMetricsProducer(osHostname string, logger *zap.Logger) *MetricsProducer {
	return &MetricsProducer{
		osHostname: osHostname,
		logger:     logger,
	}
}

// ProduceHelixPayload takes the OpenTelemetry metrics and converts them into the BMC Helix metric format
func (mp *MetricsProducer) ProduceHelixPayload(metrics pmetric.Metrics) ([]BmcHelixMetric, error) {
	helixMetrics := []BmcHelixMetric{}
	containerParentEntities := map[string]BmcHelixMetric{}

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
				newHelixMetric, err := mp.createHelixMetric(metric, resourceAttrs)
				if err != nil {
					mp.logger.Warn("Failed to create Helix metric", zap.Error(err))
					continue
				}

				helixMetrics = appendMetricWithParentEntity(helixMetrics, *newHelixMetric, containerParentEntities)
			}
		}
	}
	return helixMetrics, nil
}

// appends the metric to the helixMetrics slice and creates a parent entity if it doesn't exist
func appendMetricWithParentEntity(helixMetrics []BmcHelixMetric, helixMetric BmcHelixMetric, containerParentEntities map[string]BmcHelixMetric) []BmcHelixMetric {
	// Extract parent entity information
	parentEntityTypeID := fmt.Sprintf("%s_container", helixMetric.Labels["entityTypeId"])
	parentEntityID := fmt.Sprintf("%s:%s:%s:%s", helixMetric.Labels["source"], helixMetric.Labels["hostname"], parentEntityTypeID, parentEntityTypeID)

	// Create a parent entity if not already created
	if _, exists := containerParentEntities[parentEntityID]; !exists {
		parentMetric := BmcHelixMetric{
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
			Samples: []BmcHelixSample{}, // Parent entities don't have samples
		}
		containerParentEntities[parentEntityID] = parentMetric
		helixMetrics = append(helixMetrics, parentMetric)
	}

	// Add parent reference to the child metric
	helixMetric.Labels["parentEntityName"] = parentEntityTypeID
	helixMetric.Labels["parentEntityTypeId"] = parentEntityTypeID

	return append(helixMetrics, helixMetric)
}

// createHelixMetric converts a single OpenTelemetry metric into a BmcHelixMetric payload
func (mp *MetricsProducer) createHelixMetric(metric pmetric.Metric, resourceAttrs map[string]string) (*BmcHelixMetric, error) {
	labels := make(map[string]string)
	labels["source"] = "OTEL"

	// Add resource attributes as labels
	for k, v := range resourceAttrs {
		labels[k] = v
	}

	// Set the metric unit
	labels["unit"] = metric.Unit()

	// Set the host type
	labels["hostType"] = "server"

	// Indicates the monitor in the hierarchy that is mapped to the device
	labels["isDeviceMappingEnabled"] = "true"

	// Update the metric name for the BMC Helix payload
	labels["metricName"] = metric.Name()

	// Samples to hold the metric values
	samples := []BmcHelixSample{}

	// Handle different types of metrics (sum and gauge)
	// BMC Helix only supports simple metrics (sum, gauge, etc.) and not histograms or summaries
	switch metric.Type() {
	case pmetric.MetricTypeSum:
		dataPoints := metric.Sum().DataPoints()
		for i := 0; i < dataPoints.Len(); i++ {
			samples = mp.processDatapoint(samples, dataPoints.At(i), labels, metric, resourceAttrs)
		}
	case pmetric.MetricTypeGauge:
		dataPoints := metric.Gauge().DataPoints()
		for i := 0; i < dataPoints.Len(); i++ {
			samples = mp.processDatapoint(samples, dataPoints.At(i), labels, metric, resourceAttrs)
		}
	default:
		return nil, fmt.Errorf("unsupported metric type %s", metric.Type())
	}

	// Check if the entityTypeId is set
	if labels["entityTypeId"] == "" {
		return nil, fmt.Errorf("entityTypeId is required for the BMC Helix payload but not set for metric %s", metric.Name())
	}

	// Check if the entityName is set
	if labels["entityName"] == "" {
		return nil, fmt.Errorf("entityName is required for the BMC Helix payload but not set for metric %s", metric.Name())
	}

	return &BmcHelixMetric{
		Labels:  labels,
		Samples: samples,
	}, nil
}

// Updates the metric information for the BMC Helix payload and returns the updated samples
func (mp *MetricsProducer) processDatapoint(samples []BmcHelixSample, dp pmetric.NumberDataPoint, labels map[string]string, metric pmetric.Metric, resourceAttrs map[string]string) []BmcHelixSample {
	// Update the entity information for the BMC Helix payload
	err := mp.updateEntityInformation(labels, metric.Name(), resourceAttrs, dp.Attributes().AsRaw())
	if err != nil {
		mp.logger.Warn("Failed to update entity information", zap.Error(err))
	}

	return append(samples, newSample(dp))
}

// Update the entity information for the BMC Helix payload
func (mp *MetricsProducer) updateEntityInformation(labels map[string]string, metricName string, resourceAttrs map[string]string, dpAttributes map[string]any) error {
	// Try to get the hostname from resource attributes first
	hostname, found := resourceAttrs[conventions.AttributeHostName]
	if !found || hostname == "" {
		// Fallback to metric attributes if not found or empty in resource attributes
		if maybeHostname, ok := dpAttributes[conventions.AttributeHostName].(string); ok && maybeHostname != "" {
			hostname = maybeHostname
		} else {
			// Fallback to osHostname if hostname is not found in both places
			hostname = mp.osHostname
		}
	}

	// Add the hostname as a label (required for BMC Helix payload)
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

	// entityTypeId is required for the BMC Helix payload
	entityTypeID := stringMetricAttrs["entityTypeId"]
	if entityTypeID == "" {
		return fmt.Errorf("the entityTypeId is required for the BMC Helix payload but not set for metric %s. Metric datapoint will be skipped", metricName)
	}

	// entityName is required for the BMC Helix payload
	entityName := stringMetricAttrs["entityName"]
	if entityName == "" {
		return fmt.Errorf("the entityName is required for the BMC Helix payload but not set for metric %s. Metric datapoint will be skipped", metricName)
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

// newSample creates a new BmcHelixSample from the OpenTelemetry data point
func newSample(dp pmetric.NumberDataPoint) BmcHelixSample {
	var value float64
	switch dp.ValueType() {
	case pmetric.NumberDataPointValueTypeDouble:
		value = dp.DoubleValue()
	case pmetric.NumberDataPointValueTypeInt:
		value = float64(dp.IntValue()) // convert int to float for consistency
	}

	return BmcHelixSample{
		Value:     value,
		Timestamp: dp.Timestamp().AsTime().Unix() * 1000,
	}
}

// extractResourceAttributes extracts the resource attributes from OpenTelemetry resource data
func extractResourceAttributes(resource pcommon.Resource) map[string]string {
	attributes := make(map[string]string)

	resource.Attributes().Range(func(k string, v pcommon.Value) bool {
		attributes[k] = v.AsString()
		return true
	})

	return attributes
}
