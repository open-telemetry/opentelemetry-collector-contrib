// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewritereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusremotewritereceiver"

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	promconfig "github.com/prometheus/prometheus/config"
	writev2 "github.com/prometheus/prometheus/prompb/io/prometheus/write/v2"
	"github.com/prometheus/prometheus/storage/remote"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/prometheus/prometheus/model/labels"
)

var writeV2RequestFixture = &writev2.Request{
	Symbols: []string{"", "__name__", "test_metric1", "job", "service-x/test", "instance", "107cn001", "d", "e", "foo", "bar", "f", "g", "h", "i", "Test gauge for test purposes", "Maybe op/sec who knows (:", "Test counter for test purposes"},
	Timeseries: []writev2.TimeSeries{
		{
			Metadata:   writev2.Metadata{Type: writev2.Metadata_METRIC_TYPE_GAUGE},
			LabelsRefs: []uint32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, // Symbolized writeRequestFixture.Timeseries[0].Labels
			Samples:    []writev2.Sample{{Value: 1, Timestamp: 1}},
		},
		{
			Metadata:   writev2.Metadata{Type: writev2.Metadata_METRIC_TYPE_GAUGE},
			LabelsRefs: []uint32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, // Same series as first. Should use the same resource metrics.
			Samples:    []writev2.Sample{{Value: 2, Timestamp: 2}},
		},
		{
			Metadata:   writev2.Metadata{Type: writev2.Metadata_METRIC_TYPE_GAUGE},
			LabelsRefs: []uint32{1, 2, 3, 9, 5, 10, 7, 8, 9, 10}, // This series has different label values for job and instance.
			Samples:    []writev2.Sample{{Value: 2, Timestamp: 2}},
		},
	},
}

func setupMetricsReceiver(t *testing.T) *prometheusRemoteWriteReceiver {
	t.Helper()

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	prwReceiver, err := factory.CreateMetrics(context.Background(), receivertest.NewNopSettings(), cfg, consumertest.NewNop())
	assert.NoError(t, err)
	assert.NotNil(t, prwReceiver, "metrics receiver creation failed")

	return prwReceiver.(*prometheusRemoteWriteReceiver)
}

func TestHandlePRWContentTypeNegotiation(t *testing.T) {
	for _, tc := range []struct {
		name         string
		contentType  string
		expectedCode int
	}{
		{
			name:         "no content type",
			contentType:  "",
			expectedCode: http.StatusUnsupportedMediaType,
		},
		{
			name:         "unsupported content type",
			contentType:  "application/json",
			expectedCode: http.StatusUnsupportedMediaType,
		},
		{
			name:         "x-protobuf/no proto parameter",
			contentType:  "application/x-protobuf",
			expectedCode: http.StatusUnsupportedMediaType,
		},
		{
			name:         "x-protobuf/v1 proto parameter",
			contentType:  fmt.Sprintf("application/x-protobuf;proto=%s", promconfig.RemoteWriteProtoMsgV1),
			expectedCode: http.StatusUnsupportedMediaType,
		},
		{
			name:         "x-protobuf/v2 proto parameter",
			contentType:  fmt.Sprintf("application/x-protobuf;proto=%s", promconfig.RemoteWriteProtoMsgV2),
			expectedCode: http.StatusNoContent,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := writev2.Request{}
			pBuf := proto.NewBuffer(nil)
			err := pBuf.Marshal(&body)
			assert.NoError(t, err)

			var compressedBody []byte
			snappy.Encode(compressedBody, pBuf.Bytes())

			req := httptest.NewRequest(http.MethodPost, "/api/v1/write", bytes.NewBuffer(compressedBody))

			req.Header.Set("Content-Type", tc.contentType)
			req.Header.Set("Content-Encoding", "snappy")
			w := httptest.NewRecorder()

			prwReceiver := setupMetricsReceiver(t)
			prwReceiver.handlePRW(w, req)
			resp := w.Result()

			assert.Equal(t, tc.expectedCode, resp.StatusCode)
			if tc.expectedCode == http.StatusNoContent { // We went until the end
				assert.NotEmpty(t, resp.Header.Get("X-Prometheus-Remote-Write-Samples-Written"))
				assert.NotEmpty(t, resp.Header.Get("X-Prometheus-Remote-Write-Histograms-Written"))
				assert.NotEmpty(t, resp.Header.Get("X-Prometheus-Remote-Write-Exemplars-Written"))
			}
		})
	}
}

func TestTranslateV2(t *testing.T) {
	prwReceiver := setupMetricsReceiver(t)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	tests := []struct {
		name            string
		request         *writev2.Request
		expectError     string
		expectedMetrics pmetric.Metrics
		expectedStats   remote.WriteResponseStats
	}{
		{
			name: "duplicated scope name and version",
			request: &writev2.Request{
				Symbols: []string{
					"",
					"__name__", "test_metric",
					"job", "service-x/test",
					"instance", "107cn001",
					"otel_scope_name", "scope1",
					"otel_scope_version", "v1",
					"otel_scope_name", "scope2",
					"otel_scope_version", "v2",
					"d", "e",
					"foo", "bar",
				},
				Timeseries: []writev2.TimeSeries{
					{
						Metadata:   writev2.Metadata{Type: writev2.Metadata_METRIC_TYPE_GAUGE},
						LabelsRefs: []uint32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 15, 16},
						Samples:    []writev2.Sample{{Value: 1, Timestamp: 1}},
					},
					{
						Metadata:   writev2.Metadata{Type: writev2.Metadata_METRIC_TYPE_GAUGE},
						LabelsRefs: []uint32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 15, 16},
						Samples:    []writev2.Sample{{Value: 2, Timestamp: 2}},
					},
					{
						Metadata:   writev2.Metadata{Type: writev2.Metadata_METRIC_TYPE_GAUGE},
						LabelsRefs: []uint32{1, 2, 3, 4, 5, 6, 11, 12, 13, 14, 17, 18},
						Samples:    []writev2.Sample{{Value: 3, Timestamp: 3}},
					},
				},
			},
			expectedMetrics: func() pmetric.Metrics {
				expected := pmetric.NewMetrics()
				rm1 := expected.ResourceMetrics().AppendEmpty()
				rmAttributes1 := rm1.Resource().Attributes()
				rmAttributes1.PutStr("service.namespace", "service-x")
				rmAttributes1.PutStr("service.name", "test")
				rmAttributes1.PutStr("service.instance.id", "107cn001")
				
				sm1 := rm1.ScopeMetrics().AppendEmpty()
				sm1.Scope().SetName("scope1")
				sm1.Scope().SetVersion("v1")
				
				// First metric
				metric1 := sm1.Metrics().AppendEmpty()
				metric1.SetName("test_metric")
				gauge1 := metric1.SetEmptyGauge()
				dp1 := gauge1.DataPoints().AppendEmpty()
				dp1.SetTimestamp(pcommon.Timestamp(1 * 1_000_000))
				dp1.SetDoubleValue(1)
				dp1.Attributes().PutStr("d", "e")
				
				// Second metric
				metric2 := sm1.Metrics().AppendEmpty()
				metric2.SetName("test_metric")
				gauge2 := metric2.SetEmptyGauge()
				dp2 := gauge2.DataPoints().AppendEmpty()
				dp2.SetTimestamp(pcommon.Timestamp(2 * 1_000_000))
				dp2.SetDoubleValue(2)
				dp2.Attributes().PutStr("d", "e")
				
				// Different scope
				sm2 := rm1.ScopeMetrics().AppendEmpty()
				sm2.Scope().SetName("scope2")
				sm2.Scope().SetVersion("v2")
				metric3 := sm2.Metrics().AppendEmpty()
				metric3.SetName("test_metric")
				gauge3 := metric3.SetEmptyGauge()
				dp3 := gauge3.DataPoints().AppendEmpty()
				dp3.SetTimestamp(pcommon.Timestamp(3 * 1_000_000))
				dp3.SetDoubleValue(3)
				dp3.Attributes().PutStr("foo", "bar")
				
				return expected
			}(),
			expectedStats: remote.WriteResponseStats{},
		},
		{
			name: "missing metric name",
			request: &writev2.Request{
				Symbols: []string{"", "foo", "bar"},
				Timeseries: []writev2.TimeSeries{
					{
						LabelsRefs: []uint32{1, 2},
						Samples:    []writev2.Sample{{Value: 1, Timestamp: 1}},
					},
				},
			},
			expectError: "missing metric name in labels",
		},
		{
			name: "duplicate label",
			request: &writev2.Request{
				Symbols: []string{"", "__name__", "test"},
				Timeseries: []writev2.TimeSeries{
					{
						LabelsRefs: []uint32{1, 2, 1, 2},
						Samples:    []writev2.Sample{{Value: 1, Timestamp: 1}},
					},
				},
			},
			expectError: `duplicate label "__name__" in labels`,
		},
		{
			name:    "valid request",
			request: writeV2RequestFixture,
			expectedMetrics: func() pmetric.Metrics {
				expected := pmetric.NewMetrics()
				rm1 := expected.ResourceMetrics().AppendEmpty()
				rmAttributes1 := rm1.Resource().Attributes()
				rmAttributes1.PutStr("service.namespace", "service-x")
				rmAttributes1.PutStr("service.name", "test")
				rmAttributes1.PutStr("service.instance.id", "107cn001")
				
				sm1 := rm1.ScopeMetrics().AppendEmpty()
				
				// First metric
				metric1 := sm1.Metrics().AppendEmpty()
				metric1.SetName("test_metric1")
				gauge1 := metric1.SetEmptyGauge()
				dp1 := gauge1.DataPoints().AppendEmpty()
				dp1.SetTimestamp(pcommon.Timestamp(1 * 1_000_000))
				dp1.SetDoubleValue(1)
				dp1.Attributes().PutStr("d", "e")
				dp1.Attributes().PutStr("foo", "bar")
				
				// Second metric
				metric2 := sm1.Metrics().AppendEmpty()
				metric2.SetName("test_metric1")
				gauge2 := metric2.SetEmptyGauge()
				dp2 := gauge2.DataPoints().AppendEmpty()
				dp2.SetTimestamp(pcommon.Timestamp(2 * 1_000_000))
				dp2.SetDoubleValue(2)
				dp2.Attributes().PutStr("d", "e")
				dp2.Attributes().PutStr("foo", "bar")

				// Different resource
				rm2 := expected.ResourceMetrics().AppendEmpty()
				rmAttributes2 := rm2.Resource().Attributes()
				rmAttributes2.PutStr("service.name", "foo")
				rmAttributes2.PutStr("service.instance.id", "bar")
				
				sm2 := rm2.ScopeMetrics().AppendEmpty()
				metric3 := sm2.Metrics().AppendEmpty()
				metric3.SetName("test_metric1")
				gauge3 := metric3.SetEmptyGauge()
				dp3 := gauge3.DataPoints().AppendEmpty()
				dp3.SetTimestamp(pcommon.Timestamp(2 * 1_000_000))
				dp3.SetDoubleValue(2)
				dp3.Attributes().PutStr("d", "e")
				dp3.Attributes().PutStr("foo", "bar")

				return expected
			}(),
			expectedStats: remote.WriteResponseStats{},
		},
		{
			name: "counter metric",
			request: &writev2.Request{
				Symbols: []string{"", "job", "instance", "service-x/test", "107cn001", "__name__", "test_counter", "foo", "bar"},
				Timeseries: []writev2.TimeSeries{
					{
						Metadata: writev2.Metadata{
							Type: writev2.Metadata_METRIC_TYPE_COUNTER,
						},
						LabelsRefs: []uint32{1, 3, 2, 4, 5, 6, 7, 8},
						Samples: []writev2.Sample{
							{
								Value:     100,
								Timestamp: 1000,
							},
						},
					},
				},
			},
			expectedMetrics: func() pmetric.Metrics {
				metrics := pmetric.NewMetrics()
				rm := metrics.ResourceMetrics().AppendEmpty()
				
				// Set resource attributes
				rmAttributes := rm.Resource().Attributes()
				rmAttributes.PutStr("service.namespace", "service-x")
				rmAttributes.PutStr("service.name", "test")
				rmAttributes.PutStr("service.instance.id", "107cn001")
				
				// Create scope metrics
				sm := rm.ScopeMetrics().AppendEmpty()
				
				// Create metric
				metric := sm.Metrics().AppendEmpty()
				metric.SetName("test_counter")
				sum := metric.SetEmptySum()
				sum.SetIsMonotonic(true)
				sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				
				// Create datapoint
				dp := sum.DataPoints().AppendEmpty()
				dp.SetTimestamp(pcommon.Timestamp(1000 * 1_000_000))
				dp.SetDoubleValue(100)
				dp.Attributes().PutStr("foo", "bar")
				
				return metrics
			}(),
			expectedStats: remote.WriteResponseStats{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			metrics, stats, err := prwReceiver.translateV2(ctx, tc.request)
			if tc.expectError != "" {
				assert.ErrorContains(t, err, tc.expectError)
				return
			}

			assert.NoError(t, err)
			assert.NoError(t, pmetrictest.CompareMetrics(tc.expectedMetrics, metrics))
			assert.Equal(t, tc.expectedStats, stats)
		})
	}
}

func TestAddCounterDatapoints(t *testing.T) {
	tests := []struct {
		name     string
		labels   labels.Labels
		series   writev2.TimeSeries
		wantName string
	}{
		{
			name: "basic counter",
			labels: labels.Labels{
				{Name: "job", Value: "test_job"},
				{Name: "instance", Value: "test_instance"},
				{Name: labels.MetricName, Value: "test_counter"},
				{Name: "otel_scope_name", Value: "test_scope"},
				{Name: "otel_scope_version", Value: "v1.0"},
				{Name: "custom_label", Value: "custom_value"},
			},
			series: writev2.TimeSeries{
				Samples: []writev2.Sample{
					{Value: 42.0, Timestamp: 1234567890},
				},
				Metadata: writev2.Metadata{
					Type: writev2.Metadata_METRIC_TYPE_COUNTER,
				},
			},
			wantName: "test_counter",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metrics := pmetric.NewMetrics()
			rm := metrics.ResourceMetrics().AppendEmpty()

			addCounterDatapoints(rm, tt.labels, tt.series)

			// Verify the results
			assert.Equal(t, 1, rm.ScopeMetrics().Len())
			scope := rm.ScopeMetrics().At(0)
			assert.Equal(t, tt.labels.Get("otel_scope_name"), scope.Scope().Name())
			assert.Equal(t, tt.labels.Get("otel_scope_version"), scope.Scope().Version())

			assert.Equal(t, 1, scope.Metrics().Len())
			metric := scope.Metrics().At(0)
			
			// Verify it's a Sum (counter) metric
			assert.Equal(t, pmetric.MetricTypeSum, metric.Type())
			assert.True(t, metric.Sum().IsMonotonic())

			// Verify datapoints
			assert.Equal(t, 1, metric.Sum().DataPoints().Len())
			dp := metric.Sum().DataPoints().At(0)
			
			// Verify custom labels are present in attributes
			val, ok := dp.Attributes().Get("custom_label")
			assert.True(t, ok)
			assert.Equal(t, "custom_value", val.Str())
		})
	}
}
