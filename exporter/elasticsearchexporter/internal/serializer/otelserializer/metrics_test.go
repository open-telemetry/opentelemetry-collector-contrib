// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelserializer

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/datapoints"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/elasticsearch"
)

func TestSerializeMetricsConflict(t *testing.T) {
	metrics := pmetric.NewMetrics()
	resourceMetrics := metrics.ResourceMetrics().AppendEmpty()
	scopeMetrics := resourceMetrics.ScopeMetrics().AppendEmpty()
	var dataPoints []datapoints.DataPoint
	metric1 := scopeMetrics.Metrics().AppendEmpty()
	metric2 := scopeMetrics.Metrics().AppendEmpty()
	for _, m := range []pmetric.Metric{metric1, metric2} {
		m.SetName("foo")
		dp := m.SetEmptyGauge().DataPoints().AppendEmpty()
		dp.SetIntValue(42)
		dataPoints = append(dataPoints, datapoints.NewNumber(m, dp))
	}
	metrics.MarkReadOnly()

	var validationErrors []error
	var buf bytes.Buffer
	ser, err := New()
	require.NoError(t, err)
	_, err = ser.SerializeMetrics(resourceMetrics.Resource(), "", scopeMetrics.Scope(), "", dataPoints, &validationErrors, elasticsearch.Index{}, &buf)
	if err != nil {
		t.Errorf("Metrics() error = %v", err)
	}
	b := buf.Bytes()
	eventAsJSON := string(b)
	var result any
	decoder := json.NewDecoder(bytes.NewBuffer(b))
	decoder.UseNumber()
	if err := decoder.Decode(&result); err != nil {
		t.Error(err)
	}

	assert.Len(t, validationErrors, 1)
	assert.EqualError(t, validationErrors[0], "metric with name 'foo' has already been serialized in document with timestamp 1970-01-01T00:00:00.000000000Z")

	assert.Equal(t, map[string]any{
		"@timestamp": json.Number("0"),
		"resource":   map[string]any{},
		"scope":      map[string]any{},
		"metrics": map[string]any{
			"foo": json.Number("42"),
		},
		"_metric_names_hash": "33bf00a859c4ba3f",
	}, result, eventAsJSON)
}

func TestSerializeMetricsTemporality(t *testing.T) {
	for _, tc := range []struct {
		name         string
		temporality  pmetric.AggregationTemporality
		expectedJSON map[string]any
	}{
		{
			name:        "cumulative temporality is serialized",
			temporality: pmetric.AggregationTemporalityCumulative,
			expectedJSON: map[string]any{
				"@timestamp":  json.Number("0"),
				"resource":    map[string]any{},
				"scope":       map[string]any{},
				"temporality": "cumulative",
				"metrics": map[string]any{
					"test": map[string]any{
						"scale": json.Number("38"),
						"sum":   json.Number("20.0"),
						"min":   json.Number("10.0"),
						"max":   json.Number("100.0"),
						"zero": map[string]any{
							"count": json.Number("10"),
						},
					},
				},
				"_metric_names_hash": "4fdcca5ddb678139",
			},
		},
		{
			name:        "delta temporality is serialized",
			temporality: pmetric.AggregationTemporalityDelta,
			expectedJSON: map[string]any{
				"@timestamp":  json.Number("0"),
				"resource":    map[string]any{},
				"scope":       map[string]any{},
				"temporality": "delta",
				"metrics": map[string]any{
					"test": map[string]any{
						"scale": json.Number("38"),
						"sum":   json.Number("20.0"),
						"min":   json.Number("10.0"),
						"max":   json.Number("100.0"),
						"positive": map[string]any{
							"counts":  []any{json.Number("10")},
							"indices": []any{json.Number("274877906943")},
						},
					},
				},
				"_metric_names_hash": "4fdcca5ddb678139",
			},
		},
		{
			name:        "unspecified temporality is not serialized",
			temporality: pmetric.AggregationTemporalityUnspecified,
			expectedJSON: map[string]any{
				"@timestamp": json.Number("0"),
				"resource":   map[string]any{},
				"scope":      map[string]any{},
				"metrics": map[string]any{
					"test": map[string]any{
						"scale": json.Number("38"),
						"sum":   json.Number("20.0"),
						"min":   json.Number("10.0"),
						"max":   json.Number("100.0"),
						"positive": map[string]any{
							"counts":  []any{json.Number("10")},
							"indices": []any{json.Number("274877906943")},
						},
					},
				},
				"_metric_names_hash": "4fdcca5ddb678139",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			metrics := pmetric.NewMetrics()
			resourceMetrics := metrics.ResourceMetrics().AppendEmpty()
			scopeMetrics := resourceMetrics.ScopeMetrics().AppendEmpty()
			var dataPoints []datapoints.DataPoint
			metric := scopeMetrics.Metrics().AppendEmpty()

			metric.SetName("test")
			dp := metric.SetEmptyHistogram().DataPoints().AppendEmpty()
			dp.SetCount(10)
			dp.SetSum(20)
			dp.SetMax(100)
			dp.SetMin(10)
			metric.Histogram().SetAggregationTemporality(tc.temporality)

			dataPoints = append(dataPoints, datapoints.NewHistogram(metric, dp, datapoints.HistogramMappingExponential))

			var validationErrors []error
			var buf bytes.Buffer

			ser, err := New()
			require.NoError(t, err)

			_, err = ser.SerializeMetrics(resourceMetrics.Resource(), "", scopeMetrics.Scope(), "", dataPoints, &validationErrors, elasticsearch.Index{}, &buf)
			require.NoError(t, err, "Should not get a serialized error")

			b := buf.Bytes()
			eventAsJSON := string(b)
			var result any
			decoder := json.NewDecoder(bytes.NewBuffer(b))
			decoder.UseNumber()
			if err := decoder.Decode(&result); err != nil {
				require.NoError(t, err, "should not get a decode error")
			}

			assert.Empty(t, validationErrors)

			assert.Equal(t, tc.expectedJSON, result, eventAsJSON)
		})
	}
}
