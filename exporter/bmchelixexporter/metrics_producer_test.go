// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package bmchelixexporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

// Test for the ProduceHelixPayload method
func TestProduceHelixPayload(t *testing.T) {
	t.Parallel()

	sample := BmcHelixSample{
		Value:     42,
		Timestamp: 1634236000,
	}

	metric := BmcHelixMetric{
		Labels: map[string]string{
			"isDeviceMappingEnabled": "true",
			"entityTypeId":           "test-entity-type-id",
			"entityName":             "test-entity",
			"source":                 "OTEL",
			"unit":                   "ms",
			"hostType":               "server",
			"metricName":             "test_metric",
			"hostname":               "test-hostname",
			"instanceName":           "test-entity-Name",
			"entityId":               "OTEL:test-hostname:test-entity-type-id:test-entity",
			"parentEntityName":       "test-entity-type-id_container",
			"parentEntityTypeId":     "test-entity-type-id_container",
		},
		Samples: []BmcHelixSample{sample},
	}

	parent := BmcHelixMetric{
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
		Samples: []BmcHelixSample{},
	}

	expectedPayload := []BmcHelixMetric{parent, metric}

	producer := newMetricsProducer("test-hostname", zap.NewExample())

	tests := []struct {
		name                string
		generateMockMetrics func() pmetric.Metrics
		expectedPayload     []BmcHelixMetric
	}{
		{
			name: "SetGauge",
			generateMockMetrics: func() pmetric.Metrics {
				return generateMockMetrics(func(metric pmetric.Metric) pmetric.NumberDataPoint {
					return metric.SetEmptyGauge().DataPoints().AppendEmpty()
				})
			},
			expectedPayload: expectedPayload,
		},
		{
			name: "SetSum",
			generateMockMetrics: func() pmetric.Metrics {
				return generateMockMetrics(func(metric pmetric.Metric) pmetric.NumberDataPoint {
					return metric.SetEmptySum().DataPoints().AppendEmpty()
				})
			},
			expectedPayload: expectedPayload,
		},
		{
			name:                "emptyPayload",
			generateMockMetrics: pmetric.NewMetrics,
			expectedPayload:     []BmcHelixMetric{},
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
func generateMockMetrics(dpCreator func(metric pmetric.Metric) pmetric.NumberDataPoint) pmetric.Metrics {
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	il := rm.ScopeMetrics().AppendEmpty().Metrics()
	metric := il.AppendEmpty()
	metric.SetName("test_metric")
	metric.SetDescription("This is a test metric")
	metric.SetUnit("ms")
	dp := dpCreator(metric)
	dp.Attributes().PutStr("entityName", "test-entity")
	dp.Attributes().PutStr("entityTypeId", "test-entity-type-id")
	dp.Attributes().PutStr("instanceName", "test-entity-Name")
	dp.SetTimestamp(1634236000000000) // Example timestamp
	dp.SetDoubleValue(42.0)
	return metrics
}
