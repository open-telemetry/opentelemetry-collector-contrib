// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hydrolixexporter

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func TestDebugMetricWithTraceContext(t *testing.T) {
	// Create metrics with trace_id and span_id attributes
	metrics := pmetric.NewMetrics()
	rm := metrics.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	metric := sm.Metrics().AppendEmpty()

	metric.SetName("user_interactions")
	gauge := metric.SetEmptyGauge()
	dp := gauge.DataPoints().AppendEmpty()
	dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	dp.SetDoubleValue(1.0)

	// Add the same attributes as your payload
	dp.Attributes().PutStr("event_type", "click")
	dp.Attributes().PutStr("target_element", "SPAN.rt-Text")
	dp.Attributes().PutInt("interaction_count", 1)
	dp.Attributes().PutStr("trace_id", "fd700fd4e084e765d90a54a0412616b7")
	dp.Attributes().PutStr("span_id", "53e89fb8c3aaac01")

	rm.Resource().Attributes().PutStr("service.name", "local-cascade-frontend")

	cfg := &Config{
		ClientConfig: confighttp.ClientConfig{
			Endpoint: "http://localhost:8080",
		},
	}

	exporter := newMetricsExporter(cfg, exportertest.NewNopSettings(component.MustNewType("hydrolix")))
	result := exporter.convertToHydrolixMetrics(metrics)

	require.Len(t, result, 1)

	// Verify top-level trace context fields
	assert.Equal(t, "fd700fd4e084e765d90a54a0412616b7", result[0].TraceID)
	assert.Equal(t, "53e89fb8c3aaac01", result[0].SpanID)

	// Verify attributes are in tags
	require.Len(t, result[0].MetricAttributes, 5) // 5 attributes total

	// Convert to JSON to see the actual structure
	jsonData, err := json.MarshalIndent(result[0], "", "  ")
	require.NoError(t, err)
	t.Logf("JSON payload that would be sent to Hydrolix:\n%s", string(jsonData))

	// Verify trace_id is in the tags array
	foundTraceID := false
	foundSpanID := false
	for _, tag := range result[0].MetricAttributes {
		if val, ok := tag["trace_id"]; ok {
			foundTraceID = true
			assert.Equal(t, "fd700fd4e084e765d90a54a0412616b7", val)
		}
		if val, ok := tag["span_id"]; ok {
			foundSpanID = true
			assert.Equal(t, "53e89fb8c3aaac01", val)
		}
	}

	assert.True(t, foundTraceID, "trace_id should be in tags array")
	assert.True(t, foundSpanID, "span_id should be in tags array")
}
