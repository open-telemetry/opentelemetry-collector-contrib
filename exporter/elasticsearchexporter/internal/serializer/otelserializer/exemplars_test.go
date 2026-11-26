// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelserializer // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/serializer/otelserializer"

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/datapoints"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/internal/elasticsearch"
)

func TestSerializeExemplar(t *testing.T) {
	for _, tc := range []struct {
		name          string
		build         func() (pmetric.Metric, datapoints.DataPoint)
		expectedJSON  map[string]any
		expectedError bool
	}{
		{
			name: "serialize exemplar",
			build: func() (pmetric.Metric, datapoints.DataPoint) {
				metric := pmetric.NewMetric()
				metric.SetName("test.metric")
				metric.SetUnit("ms")
				metric.SetEmptyHistogram().SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
				dp := metric.Histogram().DataPoints().AppendEmpty()
				dp.SetSum(10)

				e := dp.Exemplars().AppendEmpty()
				e.SetDoubleValue(10)
				e.SetTraceID(pcommon.TraceID{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10})
				e.SetSpanID(pcommon.SpanID{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08})
				e.FilteredAttributes().PutInt("thread.id", 42)

				return metric, datapoints.NewHistogram(metric, dp, datapoints.HistogramMappingRaw)
			},
			expectedJSON: map[string]any{
				"@timestamp": json.Number("0"),
				"data_stream": map[string]any{
					"type":      "exemplar",
					"dataset":   "generic",
					"namespace": "default",
				},
				"resource": map[string]any{},
				"scope":    map[string]any{},
				"attributes": map[string]any{
					"state": "user",
				},
				"unit":        "ms",
				"temporality": "delta",
				"metric_name": "test.metric",
				"filtered_attributes": map[string]any{
					"thread.id": json.Number("42"),
				},
				"trace_id": "0102030405060708090a0b0c0d0e0f10",
				"span_id":  "0102030405060708",
				"value":    json.Number("10.0"),
			},
		},
		{
			name: "serialize exemplar with cumulative temporality",
			build: func() (pmetric.Metric, datapoints.DataPoint) {
				metric := pmetric.NewMetric()
				metric.SetName("test.metric")
				metric.SetUnit("ms")
				metric.SetEmptyHistogram().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				dp := metric.Histogram().DataPoints().AppendEmpty()
				dp.SetSum(10)

				e := dp.Exemplars().AppendEmpty()
				e.SetDoubleValue(10)
				e.SetTraceID(pcommon.TraceID{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10})
				e.SetSpanID(pcommon.SpanID{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08})
				e.FilteredAttributes().PutInt("thread.id", 42)

				return metric, datapoints.NewHistogram(metric, dp, datapoints.HistogramMappingRaw)
			},
			expectedJSON: map[string]any{
				"@timestamp": json.Number("0"),
				"data_stream": map[string]any{
					"type":      "exemplar",
					"dataset":   "generic",
					"namespace": "default",
				},
				"resource": map[string]any{},
				"scope":    map[string]any{},
				"attributes": map[string]any{
					"state": "user",
				},
				"unit":        "ms",
				"temporality": "cumulative",
				"metric_name": "test.metric",
				"filtered_attributes": map[string]any{
					"thread.id": json.Number("42"),
				},
				"trace_id": "0102030405060708090a0b0c0d0e0f10",
				"span_id":  "0102030405060708",
				"value":    json.Number("10.0"),
			},
		},
		{
			name: "returns error for empty exemplar",
			build: func() (pmetric.Metric, datapoints.DataPoint) {
				metric := pmetric.NewMetric()
				metric.SetName("test.metric")
				metric.SetUnit("ms")
				dp := metric.SetEmptyGauge().DataPoints().AppendEmpty()
				dp.SetDoubleValue(10)

				dp.Exemplars().AppendEmpty()
				return metric, datapoints.NewNumber(metric, dp)
			},
			expectedError: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rm := pmetric.NewMetrics().ResourceMetrics().AppendEmpty()
			sm := rm.ScopeMetrics().AppendEmpty()
			m, dp := tc.build()

			attr := pcommon.NewMap()
			attr.PutStr("state", "user")

			var buf bytes.Buffer
			buf.Grow(1024)

			ser, err := New()
			require.NoError(t, err)

			exemplar := dp.Exemplars().At(0)

			err = ser.SerializeExemplar(
				rm.Resource(),
				"",
				sm.Scope(),
				"",
				elasticsearch.NewDataStreamIndex("exemplar", "generic", "default"),
				&buf,
				exemplar,
				m,
				dp.Temporality(),
				attr,
			)
			if tc.expectedError {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)

			b := buf.Bytes()
			eventAsJSON := string(b)
			var result any
			decoder := json.NewDecoder(bytes.NewBuffer(b))
			decoder.UseNumber()
			if err := decoder.Decode(&result); err != nil {
				require.NoError(t, err, "decoder error")
			}
			assert.Equal(t, tc.expectedJSON, result, eventAsJSON)
		})
	}
}
