// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package operationsmanagement // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/bmchelixexporter/internal/operationsmanagement"

import (
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"

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
	logger           *zap.Logger
	previousCounters map[string]BMCHelixOMSample
}

// NewMetricsProducer creates a new MetricsProducer
func NewMetricsProducer(logger *zap.Logger) *MetricsProducer {
	return &MetricsProducer{
		logger:           logger,
		previousCounters: make(map[string]BMCHelixOMSample),
	}
}

// coreAttributes are label keys that should be ignored when building metric name suffixes.
var coreAttributes = map[string]struct{}{
	"source":                 {},
	"unit":                   {},
	"hostType":               {},
	"isDeviceMappingEnabled": {},
	"metricName":             {},
	"hostname":               {},
	"entityTypeId":           {},
	"entityName":             {},
	"instanceName":           {},
	"entityId":               {},
}

const rateMetricFlag = "bmchelix.requiresRateMetric"

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

				// Grow the helixMetrics slice for the new metrics
				helixMetrics = slices.Grow(helixMetrics, len(newMetrics))

				// Loop through the newly created metrics and append them to the helixMetrics slice
				// while also creating parent entities for container metrics
				for _, m := range newMetrics {
					if m.Labels["entityTypeId"] != "" {
						helixMetrics = appendMetricWithParentEntity(helixMetrics, m, containerParentEntities)
					}
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
		sliceLen := metric.Sum().DataPoints().Len()
		helixMetrics = slices.Grow(helixMetrics, sliceLen)
		for i := range sliceLen {
			dp := metric.Sum().DataPoints().At(i)
			metricPayload, err := mp.createSingleDatapointMetric(dp, metric, resourceAttrs)
			if err != nil {
				mp.logger.Warn("Failed to create Helix metric from datapoint", zap.Error(err))
				continue
			}

			// If the metric is a counter, add a flag to compute the rate metric later
			if metric.Sum().IsMonotonic() {
				metricPayload.Labels[rateMetricFlag] = "true"
			}

			helixMetrics = append(helixMetrics, *metricPayload)
		}
	case pmetric.MetricTypeGauge:
		sliceLen := metric.Gauge().DataPoints().Len()
		helixMetrics = slices.Grow(helixMetrics, sliceLen)
		for i := range sliceLen {
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

	// Enrich metric names with attributes
	// This will modify the metric names based on the attributes that have more than one distinct value
	// across the metrics with the same entityId and metricName
	// This is done to ensure that the metric names are unique and meaningful in the BMC Helix Operations Management payload
	helixMetrics = enrichMetricNamesWithAttributes(helixMetrics)

	// Add percentage variants for ratio metrics (unit "1")
	helixMetrics = addPercentageVariants(helixMetrics)

	// Compute rate metrics for counter metrics that require it
	// This will add a new metric with the same labels but with ".rate" suffix in the metric name
	// and the value being the rate of change per second
	helixMetrics = mp.addRateVariants(helixMetrics)

	return helixMetrics, nil
}

// addRateVariants checks each metric for the 'bmchelix.requiresRateMetric' label
// and computes the rate metric from the counter metric if required.
func (mp *MetricsProducer) addRateVariants(helixMetrics []BMCHelixOMMetric) []BMCHelixOMMetric {
	for _, metric := range helixMetrics {
		requiresRate := metric.Labels[rateMetricFlag] == "true"
		if !requiresRate {
			continue
		}

		// Compute the rate metric from the counter metric
		if rateMetric := mp.computeRateMetricFromCounter(metric); rateMetric != nil {
			// Add the rate metric to the helixMetrics slice
			helixMetrics = append(helixMetrics, *rateMetric)
		}

		// Remove the 'bmchelix.requiresRateMetric' label
		delete(metric.Labels, rateMetricFlag)
	}
	return helixMetrics
}

// createSingleDatapointMetric creates a single BMCHelixOMMetric from a single OpenTelemetry datapoint
func (mp *MetricsProducer) createSingleDatapointMetric(dp pmetric.NumberDataPoint, metric pmetric.Metric, resourceAttrs map[string]string) (*BMCHelixOMMetric, error) {
	labels := make(map[string]string)
	labels["source"] = "OTEL"

	// Add resource attributes
	maps.Copy(labels, resourceAttrs)

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
func (*MetricsProducer) updateEntityInformation(labels map[string]string, metricName string, resourceAttrs map[string]string, dpAttributes map[string]any) error {
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
	maps.Copy(stringMetricAttrs, resourceAttrs)

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

	// Trim trailing and leading colons from entityName
	entityName = strings.Trim(entityName, ":")

	// Remove any colons from the entityName to ensure compatibility with BMC Helix Operations Management
	entityName = strings.ReplaceAll(entityName, ":", "")

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

// enrichMetricNamesWithAttributes modifies the metric names by appending distinguishing attributes
// that have more than one distinct value across the metrics with the same entityId and metricName
// A copy of the metric is created without entityId, entityTypeId, and entityName attributes
// to ensure that the original metric is preserved for the BMC Helix VictoriaMetrics.
// This is done to ensure that the metric names are unique in the BMC Helix Operations Management payload
func enrichMetricNamesWithAttributes(metrics []BMCHelixOMMetric) []BMCHelixOMMetric {
	// Step 1: Group metrics by (entityId + metricName)
	groups := make(map[string][]*BMCHelixOMMetric)
	for i := range metrics {
		m := &metrics[i]
		key := m.Labels["entityId"] + ":" + m.Labels["metricName"]
		groups[key] = append(groups[key], m)
	}

	finalMetrics := make([]BMCHelixOMMetric, 0, len(metrics)*2)

	// Step 2: Process each group
	for _, group := range groups {
		attrValues := make(map[string]map[string]struct{})

		// Collect all attribute values for each key
		for _, m := range group {
			for k, v := range m.Labels {
				if _, shouldSkip := coreAttributes[k]; shouldSkip {
					continue
				}
				if _, exists := attrValues[k]; !exists {
					attrValues[k] = make(map[string]struct{})
				}
				attrValues[k][v] = struct{}{}
			}
		}

		// Step 3: Identifying attributes (those with >1 distinct value)
		var identifyingKeys []string
		for k, vals := range attrValues {
			if len(vals) > 1 {
				identifyingKeys = insertSorted(identifyingKeys, k)
			}
		}

		// Step 4: Modify metric names by appending attribute values
		for _, m := range group {
			originalMetricName := m.Labels["metricName"]

			// Build suffix
			var suffixParts []string
			for _, attrKey := range identifyingKeys {
				if val, ok := m.Labels[attrKey]; ok {
					suffixParts = append(suffixParts, val)
				}
			}

			// Only create copy + modify metric if there's a suffix to apply
			if len(suffixParts) > 0 {
				// Step 1: Raw copy without entityId/entityTypeId/entityName
				rawCopy := BMCHelixOMMetric{
					Labels:  make(map[string]string),
					Samples: m.Samples,
				}
				for k, v := range m.Labels {
					if k != "entityId" && k != "entityTypeId" && k != "entityName" {
						rawCopy.Labels[k] = v
					}
				}
				rawCopy.Labels["metricName"] = originalMetricName
				finalMetrics = append(finalMetrics, rawCopy)

				// Step 2: Modify the original metric
				m.Labels["metricName"] = originalMetricName + "." + strings.Join(suffixParts, ".")
				for _, attrKey := range identifyingKeys {
					delete(m.Labels, attrKey) // Remove identifying attributes from the metric labels
				}
			}

			// Always keep the (possibly modified) original metric
			finalMetrics = append(finalMetrics, *m)
		}
	}

	return finalMetrics
}

// Binary-inserted sorted slice
func insertSorted(keys []string, key string) []string {
	idx := sort.SearchStrings(keys, key) // find insertion index
	keys = append(keys, "")              // grow the slice by 1
	copy(keys[idx+1:], keys[idx:])       // shift right to make room
	keys[idx] = key                      // insert the new key
	return keys
}

// addPercentageVariants adds percentage variants of metrics that are ratios (unit "1")
// This is done to ensure that the BMC Helix Operations Management payload contains both the original
// ratio metric and its percentage variant, which is often useful for visualization and analysis.
func addPercentageVariants(metrics []BMCHelixOMMetric) []BMCHelixOMMetric {
	final := make([]BMCHelixOMMetric, 0, len(metrics)*2)

	for _, m := range metrics {
		final = append(final, m)

		unit := m.Labels["unit"]
		if unit != "1" {
			continue // Not a ratio
		}

		// Clone the original
		percentLabels := make(map[string]string, len(m.Labels))
		maps.Copy(percentLabels, m.Labels)

		// Rename metricName
		originalName := percentLabels["metricName"]

		percentLabels["metricName"] = toPercentMetricName(originalName)
		percentLabels["unit"] = "%"

		// Convert sample value
		percentSamples := make([]BMCHelixOMSample, len(m.Samples))
		for i, s := range m.Samples {
			percentSamples[i] = BMCHelixOMSample{
				Value:     s.Value * 100,
				Timestamp: s.Timestamp,
			}
		}

		final = append(final, BMCHelixOMMetric{
			Labels:  percentLabels,
			Samples: percentSamples,
		})
	}

	return final
}

// toPercentMetricName converts a metric name to its percentage variant
func toPercentMetricName(originalName string) string {
	if strings.HasSuffix(originalName, ".percent") {
		return originalName // already transformed
	}

	if strings.HasSuffix(originalName, "ratio") {
		return strings.TrimSuffix(originalName, "ratio") + "percent"
	}

	return originalName + ".percent"
}

// computeRateMetricFromCounter computes a rate metric from a counter metric
func (mp *MetricsProducer) computeRateMetricFromCounter(metric BMCHelixOMMetric) *BMCHelixOMMetric {
	if len(metric.Samples) != 1 {
		return nil
	}

	sample := metric.Samples[0]
	key := metric.Labels["entityId"] + ":" + metric.Labels["metricName"]

	prev, ok := mp.previousCounters[key]
	mp.previousCounters[key] = sample

	if !ok || sample.Timestamp <= prev.Timestamp {
		return nil // not enough data
	}

	deltaValue := sample.Value - prev.Value
	if deltaValue < 0 {
		mp.logger.Debug("Negative delta value, resetting to zero", zap.String("key", key), zap.Float64("deltaValue", deltaValue))
		deltaValue = 0 // Avoid negative rates
	}

	deltaTime := float64(sample.Timestamp-prev.Timestamp) / 1000.0 // ms to sec

	if deltaTime <= 0 {
		mp.logger.Debug("Zero or negative delta time, skipping rate calculation", zap.String("key", key), zap.Float64("deltaTime", deltaTime))
		return nil
	}

	rate := deltaValue / deltaTime

	// Clone labels
	rateLabels := make(map[string]string, len(metric.Labels))
	for k, v := range metric.Labels {
		if k != rateMetricFlag {
			rateLabels[k] = v
		}
	}

	// Modify metric name and unit for rate
	rateLabels["metricName"] += ".rate"
	rateLabels["unit"] += "/s"

	return &BMCHelixOMMetric{
		Labels:  rateLabels,
		Samples: []BMCHelixOMSample{{Value: rate, Timestamp: sample.Timestamp}},
	}
}
