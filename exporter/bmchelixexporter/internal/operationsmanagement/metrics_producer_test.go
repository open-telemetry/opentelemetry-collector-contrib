// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package operationsmanagement

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pmetric"
	conventions "go.opentelemetry.io/otel/semconv/v1.27.0"
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

	producer := NewMetricsProducer(zap.NewExample())

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

			assert.Equal(t, tt.expectedPayload, payload, "Payload should match the expected payload")
		})
	}
}

// Mock data generation for testing
func generateMockMetrics(setMetricType func(metric pmetric.Metric) pmetric.NumberDataPointSlice) pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	il := rm.ScopeMetrics().AppendEmpty().Metrics()
	metric := il.AppendEmpty()
	metric.SetName("test_metric")
	metric.SetDescription("This is a test metric")
	metric.SetUnit("s")

	dps := setMetricType(metric) // only call this once!

	// First datapoint
	dp1 := dps.AppendEmpty()
	dp1.Attributes().PutStr(string(conventions.HostNameKey), "test-hostname")
	dp1.Attributes().PutStr("entityName", "test-entity-1")
	dp1.Attributes().PutStr("entityTypeId", "test-entity-type-id")
	dp1.Attributes().PutStr("instanceName", "test-entity-Name-1")
	dp1.SetTimestamp(1750926531000000000)
	dp1.SetDoubleValue(42.0)

	// Second datapoint
	dp2 := dps.AppendEmpty()
	dp2.Attributes().PutStr(string(conventions.HostNameKey), "test-hostname")
	dp2.Attributes().PutStr("entityName", "test-entity-2")
	dp2.Attributes().PutStr("entityTypeId", "test-entity-type-id")
	dp2.Attributes().PutStr("instanceName", "test-entity-Name-2")
	dp2.SetTimestamp(1750926532000000000)
	dp2.SetDoubleValue(84.0)

	return metrics
}


