// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelserializer

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
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
	ser := New()
	_, err := ser.SerializeMetrics(resourceMetrics.Resource(), "", scopeMetrics.Scope(), "", dataPoints, &validationErrors, elasticsearch.Index{}, &buf)
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
		"@timestamp": "0.0",
		"resource":   map[string]any{},
		"scope":      map[string]any{},
		"metrics": map[string]any{
			"foo": json.Number("42"),
		},
		"_metric_names_hash": "a9f37ed7",
	}, result, eventAsJSON)
}
