// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package operationsmanagement

import (
	"maps"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

// Test for the ProduceHelixPayload method
func TestProduceHelixPayload(t *testing.T) {
	t.Parallel()

	sample1 := BMCHelixOMSample{Value: 42, Timestamp: 1750926531000}
	sample2 := BMCHelixOMSample{Value: 84, Timestamp: 1750926532000}

	metric1 := BMCHelixOMMetric{
		Labels: map[string]string{
			"isDeviceMappingEnabled": "true",
			"entityTypeId":           "test-entity-type-id",
			"entityName":             "test-entity-1",
			"source":                 "OTEL",
			"unit":                   "s",
			"hostType":               "server",
			"metricName":             "test_metric",
			"hostname":               "test-hostname",
			"instanceName":           "test-entity-Name-1",
			"entityId":               "OTEL:test-hostname:test-entity-type-id:test-entity-1",
			"parentEntityName":       "test-entity-type-id_container",
			"parentEntityTypeId":     "test-entity-type-id_container",
			"host.name":              "test-hostname",
		},
		Samples: []BMCHelixOMSample{sample1},
	}

	metric2 := BMCHelixOMMetric{
		Labels: map[string]string{
			"isDeviceMappingEnabled": "true",
			"entityTypeId":           "test-entity-type-id",
			"entityName":             "test-entity-2",
			"source":                 "OTEL",
			"unit":                   "s",
			"hostType":               "server",
			"metricName":             "test_metric",
			"hostname":               "test-hostname",
			"instanceName":           "test-entity-Name-2",
			"entityId":               "OTEL:test-hostname:test-entity-type-id:test-entity-2",
			"parentEntityName":       "test-entity-type-id_container",
			"parentEntityTypeId":     "test-entity-type-id_container",
			"host.name":              "test-hostname",
		},
		Samples: []BMCHelixOMSample{sample2},
	}

	parent := BMCHelixOMMetric{
		Labels: map[string]string{
			"entityTypeId":           "test-entity-type-id_container",
			"entityName":             "test-entity-type-id_container",
			"isDeviceMappingEnabled": "true",
			"source":                 "OTEL",
			"hostType":               "server",
			"hostname":               "test-hostname",
			"entityId":               "OTEL:test-hostname:test-entity-type-id_container:test-entity-type-id_container",
			"metricName":             "identity",
		},
		Samples: []BMCHelixOMSample{},
	}

	expectedPayload := []BMCHelixOMMetric{parent, metric1, metric2}

	producer := NewMetricsProducer(zap.NewExample(), true)

	tests := []struct {
		name                string
		generateMockMetrics func() pmetric.Metrics
		expectedPayload     []BMCHelixOMMetric
	}{
		{
			name: "SetGauge",
			generateMockMetrics: func() pmetric.Metrics {
				return generateMockMetrics(func(metric pmetric.Metric) pmetric.NumberDataPointSlice {
					return metric.SetEmptyGauge().DataPoints()
				})
			},
			expectedPayload: expectedPayload,
		},
		{
			name: "SetSum",
			generateMockMetrics: func() pmetric.Metrics {
				return generateMockMetrics(func(metric pmetric.Metric) pmetric.NumberDataPointSlice {
					return metric.SetEmptySum().DataPoints()
				})
			},
			expectedPayload: expectedPayload,
		},
		{
			name:                "emptyPayload",
			generateMockMetrics: pmetric.NewMetrics,
			expectedPayload:     []BMCHelixOMMetric{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockMetrics := tt.generateMockMetrics()
			payload, err := producer.ProduceHelixPayload(mockMetrics)
			assert.NoError(t, err, "Expected no error during payload production")
			assert.NotNil(t, payload, "Payload should not be nil")

			assert.ElementsMatch(t, tt.expectedPayload, payload, "Payload should match the expected payload")
		})
	}
}

// TestProduceHelixPayloadWithEnrichmentDisabled verifies that when enrichMetricWithAttributes is false,
// no enriched metrics are created and the original metrics retain their entityId.
func TestProduceHelixPayloadWithEnrichmentDisabled(t *testing.T) {
	t.Parallel()

	producer := NewMetricsProducer(zap.NewExample(), false)

	// Generate metrics with non-core attributes that would normally trigger enrichment
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("host.name", "test-hostname")
	sm := rm.ScopeMetrics().AppendEmpty()
	metric := sm.Metrics().AppendEmpty()
	metric.SetName("system.cpu.time")
	metric.SetUnit("s")

	dp := metric.SetEmptyGauge().DataPoints().AppendEmpty()
	dp.SetTimestamp(1750926531000000000)
	dp.SetDoubleValue(42.0)
	dp.Attributes().PutStr("entityTypeId", "cpu")
	dp.Attributes().PutStr("entityName", "cpu0")
	dp.Attributes().PutStr("cpu.mode", "idle") // Non-core attribute that would trigger enrichment

	payload, err := producer.ProduceHelixPayload(metrics)
	assert.NoError(t, err)

	// With enrichment disabled, we should get:
	// 1. Parent entity (identity metric)
	// 2. Original metric with entityId preserved (no enriched variant)
	// Total: 2 metrics (not 3 as would be with enrichment enabled)
	assert.Len(t, payload, 2, "Should have parent + original metric only, no enriched variant")

	// Find the actual metric (not the parent identity)
	var actualMetric *BMCHelixOMMetric
	for i := range payload {
		if payload[i].Labels["metricName"] != "identity" {
			actualMetric = &payload[i]
			break
		}
	}

	assert.NotNil(t, actualMetric, "Should have found the actual metric")
	assert.Equal(t, "system.cpu.time", actualMetric.Labels["metricName"], "Metric name should not be enriched")
	assert.NotEmpty(t, actualMetric.Labels["entityId"], "entityId should be preserved when enrichment is disabled")
}

// Mock data generation for testing
func generateMockMetrics(setMetricType func(metric pmetric.Metric) pmetric.NumberDataPointSlice) pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("host.name", "test-hostname")
	il := rm.ScopeMetrics().AppendEmpty().Metrics()
	metric := il.AppendEmpty()
	metric.SetName("test_metric")
	metric.SetDescription("This is a test metric")
	metric.SetUnit("s")

	dps := setMetricType(metric) // only call this once!

	// First datapoint
	dp1 := dps.AppendEmpty()
	dp1.Attributes().PutStr("entityName", "test-entity-1")
	dp1.Attributes().PutStr("entityTypeId", "test-entity-type-id")
	dp1.Attributes().PutStr("instanceName", "test-entity-Name-1")
	dp1.SetTimestamp(1750926531000000000)
	dp1.SetDoubleValue(42.0)

	// Second datapoint
	dp2 := dps.AppendEmpty()
	dp2.Attributes().PutStr("entityName", "test-entity-2")
	dp2.Attributes().PutStr("entityTypeId", "test-entity-type-id")
	dp2.Attributes().PutStr("instanceName", "test-entity-Name-2")
	dp2.SetTimestamp(1750926532000000000)
	dp2.SetDoubleValue(84.0)

	return metrics
}

func TestCreateEnrichedMetricWithDpAttributes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name               string
		inputMetric        *BMCHelixOMMetric
		dpAttrs            map[string]any
		expectedMetricName string
		expectNil          bool
	}{
		{
			name: "No non-core attributes returns nil",
			inputMetric: &BMCHelixOMMetric{
				Labels: map[string]string{
					"entityId":   "host:cpu:core0",
					"metricName": "system.cpu.time",
					"hostname":   "myhost",
				},
			},
			dpAttrs: map[string]any{
				"hostname": "myhost", // core attribute only
			},
			expectNil: true,
		},
		{
			name: "Single non-core attribute appends normalized value",
			inputMetric: &BMCHelixOMMetric{
				Labels: map[string]string{
					"entityId":   "host:cpu:core0",
					"metricName": "system.cpu.time",
				},
			},
			dpAttrs: map[string]any{
				"cpu.mode": "idle",
			},
			expectedMetricName: "system.cpu.time.idle",
		},
		{
			name: "Multiple non-core attributes sorted by key",
			inputMetric: &BMCHelixOMMetric{
				Labels: map[string]string{
					"entityId":   "host:cpu:core0",
					"metricName": "system.cpu.time",
				},
			},
			dpAttrs: map[string]any{
				"cpu.mode": "user",
				"cpu.core": "0",
			},
			expectedMetricName: "system.cpu.time.0.user", // cpu.core < cpu.mode
		},
		{
			name: "Attribute values are normalized",
			inputMetric: &BMCHelixOMMetric{
				Labels: map[string]string{
					"metricName": "my.metric",
				},
			},
			dpAttrs: map[string]any{
				"state": "Read/Write (Active)",
			},
			expectedMetricName: "my.metric.Read_Write_Active_",
		},
		{
			name: "Empty and nil attribute values are skipped",
			inputMetric: &BMCHelixOMMetric{
				Labels: map[string]string{
					"metricName": "my.metric",
				},
			},
			dpAttrs: map[string]any{
				"valid": "ok",
				"empty": "",
				"nil":   nil,
			},
			expectedMetricName: "my.metric.ok",
		},
		{
			name: "All attributes empty or nil returns nil",
			inputMetric: &BMCHelixOMMetric{
				Labels: map[string]string{
					"metricName": "my.metric",
				},
			},
			dpAttrs: map[string]any{
				"empty": "",
				"nil":   nil,
			},
			expectNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Snapshot input labels before the call to verify immutability
			originalLabels := make(map[string]string, len(tt.inputMetric.Labels))
			maps.Copy(originalLabels, tt.inputMetric.Labels)

			result := createEnrichedMetricWithDpAttributes(tt.inputMetric, tt.dpAttrs)

			// Ensure the input metric is not mutated (including entityId and all other labels)
			assert.Equal(t, originalLabels, tt.inputMetric.Labels, "input metric labels must not be mutated")

			if tt.expectNil {
				assert.Nil(t, result)
				return
			}

			assert.NotNil(t, result)
			assert.Equal(t, tt.expectedMetricName, result.Labels["metricName"])
			// Ensure the enriched metric name differs from the original
			assert.NotEqual(t, originalLabels["metricName"], result.Labels["metricName"])
		})
	}
}

func TestComputeRateMetricFromCounter(t *testing.T) {
	t.Parallel()
	producer := &MetricsProducer{
		logger:           nil,
		previousCounters: make(map[string]BMCHelixOMSample),
	}

	labels := map[string]string{
		"entityId":   "OTEL:host:network:eth0",
		"metricName": "hw.network.io",
		"unit":       "By",
		"hostname":   "host",
		"source":     "OTEL",
	}

	// First sample: initial counter datapoint
	now := time.Now()
	first := BMCHelixOMMetric{
		Labels: labels,
		Samples: []BMCHelixOMSample{{
			Value:     5000,
			Timestamp: now.UnixMilli(),
		}},
	}

	// First call – no rate should be returned yet
	assert.Nil(t, producer.computeRateMetricFromCounter(first), "First datapoint should not yield a rate metric")

	// Simulate a second datapoint after a short delay (simulate time passage)
	next := now.Add(100 * time.Millisecond)

	second := BMCHelixOMMetric{
		Labels: labels,
		Samples: []BMCHelixOMSample{{
			Value:     8000,
			Timestamp: next.UnixMilli(),
		}},
	}

	rateMetric := producer.computeRateMetricFromCounter(second)
	assert.NotNil(t, rateMetric, "Expected a rate metric on second datapoint")

	assert.Equal(t, "hw.network.io.rate", rateMetric.Labels["metricName"])
	assert.Equal(t, "By/s", rateMetric.Labels["unit"])
	assert.Len(t, rateMetric.Samples, 1)
	assert.Greater(t, rateMetric.Samples[0].Value, 0.0, "Rate should be a positive value")
}

func TestToPercentMetricName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		original     string
		expectedName string
	}{
		{
			name:         "ends with ratio",
			original:     "hw.fan.ratio",
			expectedName: "hw.fan.percent",
		},
		{
			name:         "contains ratio mid-word",
			original:     "some.ratio.metric.value",
			expectedName: "some.ratio.metric.value.percent",
		},
		{
			name:         "no ratio present",
			original:     "disk.utilization",
			expectedName: "disk.utilization.percent",
		},
		{
			name:         "already ends with .percent",
			original:     "network.bandwidth.percent",
			expectedName: "network.bandwidth.percent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toPercentMetricName(tt.original)
			assert.Equal(t, tt.expectedName, got)
		})
	}
}

func TestAddPercentageVariants(t *testing.T) {
	t.Parallel()
	ratioLabels := map[string]string{
		"metricName": "hw.network.ratio",
		"unit":       "1",
		"entityId":   "OTEL:host:network:eth0",
		"hostname":   "host",
		"source":     "OTEL",
	}

	sample := BMCHelixOMSample{
		Value:     0.82,
		Timestamp: 1690000000000,
	}

	metrics := []BMCHelixOMMetric{
		{
			Labels:  ratioLabels,
			Samples: []BMCHelixOMSample{sample},
		},
	}

	result := addPercentageVariants(metrics)
	assert.Len(t, result, 2, "Expected original + .percent variant")

	// Original preserved
	original := result[0]
	assert.Equal(t, "hw.network.ratio", original.Labels["metricName"])
	assert.Equal(t, "1", original.Labels["unit"])
	assert.Equal(t, 0.82, original.Samples[0].Value)

	// Percent variant added
	percent := result[1]
	assert.Equal(t, "hw.network.percent", percent.Labels["metricName"])
	assert.Equal(t, "%", percent.Labels["unit"])
	assert.InDelta(t, 82.0, percent.Samples[0].Value, 0.001)
	assert.Equal(t, sample.Timestamp, percent.Samples[0].Timestamp)
}

func TestAddRateVariants(t *testing.T) {
	t.Parallel()

	producer := NewMetricsProducer(zap.NewExample(), true)

	// Create a base counter metric
	originalLabels := map[string]string{
		"metricName":   "hw.network.io",
		"unit":         "By",
		"entityId":     "OTEL:host:network:eth0",
		"hostname":     "host",
		"source":       "OTEL",
		rateMetricFlag: "true",
	}

	t1 := time.Now().UnixMilli()
	sample1 := BMCHelixOMSample{Value: 1000, Timestamp: t1}

	// Store the first sample for comparison (simulate already seen)
	producer.previousCounters["OTEL:host:network:eth0:hw.network.io"] = sample1

	// Second sample (incoming)
	t2 := t1 + 1000 // 1 second later
	sample2 := BMCHelixOMSample{Value: 2000, Timestamp: t2}

	inputMetric := BMCHelixOMMetric{
		Labels:  originalLabels,
		Samples: []BMCHelixOMSample{sample2},
	}

	// Run addRateVariants
	metrics := producer.addRateVariants([]BMCHelixOMMetric{inputMetric})

	assert.Len(t, metrics, 2, "Should return original + rate metric")

	// Original metric should remain unchanged except the flag
	orig := metrics[0]
	_, exists := orig.Labels[rateMetricFlag]
	assert.False(t, exists, "Temporary label should be removed after processing")

	// Check the added rate metric
	rate := metrics[1]
	assert.Equal(t, "hw.network.io.rate", rate.Labels["metricName"])
	assert.Equal(t, "By/s", rate.Labels["unit"])
	assert.Len(t, rate.Samples, 1)

	expectedRate := 1000.0 // (2000 - 1000) / 1s
	assert.InDelta(t, expectedRate, rate.Samples[0].Value, 0.001)
	assert.Equal(t, t2, rate.Samples[0].Timestamp)
}

func TestEmptyMetricNameSkipsPayload(t *testing.T) {
	t.Parallel()

	producer := NewMetricsProducer(zap.NewExample(), true)

	// Test 1: Metric with empty name (normalizes to empty string and should be skipped)
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	resource := rm.Resource()
	resource.Attributes().PutStr("host.name", "test-host")

	sm := rm.ScopeMetrics().AppendEmpty()
	metric := sm.Metrics().AppendEmpty()
	metric.SetName("") // Empty name normalizes to empty and should be skipped

	gauge := metric.SetEmptyGauge()
	dp := gauge.DataPoints().AppendEmpty()
	dp.SetTimestamp(1750926531000000000)
	dp.SetDoubleValue(42.0)
	dp.Attributes().PutStr("entityTypeId", "test-entity")
	dp.Attributes().PutStr("entityName", "test-name")

	payload, err := producer.ProduceHelixPayload(metrics)

	// Should not return an error, but the payload should be empty since the metric was skipped
	assert.NoError(t, err)
	assert.Empty(t, payload, "Metrics with empty normalized names should be skipped")
}

func TestCreateEnrichedMetricWithEmptyName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		metricName  string
		dpAttrs     map[string]any
		expectNil   bool
		description string
	}{
		{
			name:       "Enriched metric name becomes empty",
			metricName: "valid.metric",
			dpAttrs: map[string]any{
				"attribute": "!@#$%", // This will normalize to empty, making the whole enriched name potentially empty
			},
			expectNil:   false, // The enriched name would be "valid.metric." which is not empty
			description: "Even if attribute value normalizes poorly, base metric name keeps it non-empty",
		},
		{
			name:       "Base metric name empty, enrichment attempted",
			metricName: "", // Empty base metric name
			dpAttrs: map[string]any{
				"attribute": "value",
			},
			expectNil:   false, // Will create ".value" which normalizes to "value"
			description: "Empty base with valid attribute creates enriched metric",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inputMetric := &BMCHelixOMMetric{
				Labels: map[string]string{
					"metricName": tt.metricName,
					"entityId":   "test:entity:id",
				},
				Samples: []BMCHelixOMSample{{Value: 1.0, Timestamp: 1000}},
			}

			result := createEnrichedMetricWithDpAttributes(inputMetric, tt.dpAttrs)

			if tt.expectNil {
				assert.Nil(t, result, tt.description)
			} else {
				assert.NotNil(t, result, tt.description)
				if result != nil {
					// Verify the metric name is not empty
					assert.NotEmpty(t, result.Labels["metricName"], "Enriched metric name should not be empty")
				}
			}
		})
	}
}
