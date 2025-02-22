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
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	promconfig "github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/model/labels"
	writev2 "github.com/prometheus/prometheus/prompb/io/prometheus/write/v2"
	"github.com/prometheus/prometheus/storage/remote"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusremotewritereceiver/internal/metadata"
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

	prwReceiver, err := factory.CreateMetrics(context.Background(), receivertest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
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

	for _, tc := range []struct {
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

				dp1 := sm1.Metrics().AppendEmpty().SetEmptyGauge().DataPoints().AppendEmpty()
				dp1.SetTimestamp(pcommon.Timestamp(1 * int64(time.Millisecond)))
				dp1.SetDoubleValue(1.0)
				dp1.Attributes().PutStr("d", "e")

				dp2 := sm1.Metrics().AppendEmpty().SetEmptyGauge().DataPoints().AppendEmpty()
				dp2.SetTimestamp(pcommon.Timestamp(2 * int64(time.Millisecond)))
				dp2.SetDoubleValue(2.0)
				dp2.Attributes().PutStr("d", "e")

				sm2 := rm1.ScopeMetrics().AppendEmpty()
				sm2.Scope().SetName("scope2")
				sm2.Scope().SetVersion("v2")

				dp3 := sm2.Metrics().AppendEmpty().SetEmptyGauge().DataPoints().AppendEmpty()
				dp3.SetTimestamp(pcommon.Timestamp(3 * int64(time.Millisecond)))
				dp3.SetDoubleValue(3.0)
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
				dp1 := sm1.Metrics().AppendEmpty().SetEmptyGauge().DataPoints().AppendEmpty()
				dp1.SetTimestamp(pcommon.Timestamp(1 * int64(time.Millisecond)))
				dp1.SetDoubleValue(1.0)
				dp1.Attributes().PutStr("d", "e")
				dp1.Attributes().PutStr("foo", "bar")

				dp2 := sm1.Metrics().AppendEmpty().SetEmptyGauge().DataPoints().AppendEmpty()
				dp2.SetTimestamp(pcommon.Timestamp(2 * int64(time.Millisecond)))
				dp2.SetDoubleValue(2.0)
				dp2.Attributes().PutStr("d", "e")
				dp2.Attributes().PutStr("foo", "bar")

				rm2 := expected.ResourceMetrics().AppendEmpty()
				rmAttributes2 := rm2.Resource().Attributes()
				rmAttributes2.PutStr("service.name", "foo")
				rmAttributes2.PutStr("service.instance.id", "bar")

				dp3 := rm2.ScopeMetrics().AppendEmpty().Metrics().AppendEmpty().SetEmptyGauge().DataPoints().AppendEmpty()
				dp3.SetTimestamp(pcommon.Timestamp(2 * int64(time.Millisecond)))
				dp3.SetDoubleValue(2.0)
				dp3.Attributes().PutStr("d", "e")
				dp3.Attributes().PutStr("foo", "bar")

				return expected
			}(),
			expectedStats: remote.WriteResponseStats{},
		},
	} {
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

func TestAddHistogramDatapoints(t *testing.T) {
	tests := []struct {
		name     string
		labels   labels.Labels
		series   writev2.TimeSeries
		validate func(t *testing.T, rm pmetric.ResourceMetrics)
	}{
		{
			name: "basic histogram with positive values",
			labels: labels.Labels{
				{Name: "otel_scope_name", Value: "test_scope"},
				{Name: "otel_scope_version", Value: "v1.0"},
				{Name: "job", Value: "test_job"},
				{Name: "instance", Value: "test_instance"},
				{Name: labels.MetricName, Value: "test_histogram"},
				{Name: "custom_label", Value: "custom_value"},
			},
			series: writev2.TimeSeries{
				Histograms: []writev2.Histogram{
					{
						Count:          &writev2.Histogram_CountFloat{CountFloat: 100},
						Sum:            50.0,
						Schema:         1,
						ZeroThreshold:  0.0001,
						ZeroCount:      &writev2.Histogram_ZeroCountFloat{ZeroCountFloat: 10},
						PositiveSpans:  []writev2.BucketSpan{{Offset: 0, Length: 2}},
						PositiveCounts: []float64{30, 60},
						Timestamp:      123456789,
					},
				},
			},
			validate: func(t *testing.T, rm pmetric.ResourceMetrics) {
				// Validate scope metrics
				assert.Equal(t, 1, rm.ScopeMetrics().Len())
				scope := rm.ScopeMetrics().At(0)
				assert.Equal(t, "test_scope", scope.Scope().Name())
				assert.Equal(t, "v1.0", scope.Scope().Version())

				// Validate metrics
				assert.Equal(t, 1, scope.Metrics().Len())
				metric := scope.Metrics().At(0)
				assert.Equal(t, "test_histogram", metric.Name())
				assert.Equal(t, pmetric.MetricTypeHistogram, metric.Type())

				// Validate histogram data points
				histogram := metric.Histogram()
				assert.Equal(t, 1, histogram.DataPoints().Len())
				dp := histogram.DataPoints().At(0)

				// Validate timestamp
				assert.Equal(t, pcommon.Timestamp(123456789), dp.Timestamp())

				// Validate counts and sum
				assert.Equal(t, uint64(100), dp.Count())
				assert.Equal(t, 50.0, dp.Sum())

				// Validate buckets
				expectedBounds := []float64{2.0, 4.0}  // 2^1 and 2^2
				expectedCounts := []uint64{30, 60, 10} // Last 10 is zero count

				assert.Equal(t, expectedBounds, dp.ExplicitBounds().AsRaw())
				assert.Equal(t, expectedCounts, dp.BucketCounts().AsRaw())

				// Validate attributes
				attrs := dp.Attributes()
				val, exists := attrs.Get("custom_label")
				assert.True(t, exists)
				assert.Equal(t, "custom_value", val.AsString())

				// Verify system labels are not included
				_, exists = attrs.Get("job")
				assert.False(t, exists)
				_, exists = attrs.Get("instance")
				assert.False(t, exists)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a new Metrics instance
			metricsData := pmetric.NewMetrics()
			rm := metricsData.ResourceMetrics().AppendEmpty()

			// Initialize scope metrics first
			sm := rm.ScopeMetrics().AppendEmpty()
			sm.Scope().SetName(tt.labels.Get("otel_scope_name"))
			sm.Scope().SetVersion(tt.labels.Get("otel_scope_version"))

			addHistogramDatapoints(rm, tt.labels, tt.series)
			tt.validate(t, rm)
		})
	}
}
