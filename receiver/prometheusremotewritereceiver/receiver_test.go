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
			name: "UnitRef bigger than symbols length",
			request: &writev2.Request{
				Symbols: []string{"", "__name__", "test"},
				Timeseries: []writev2.TimeSeries{
					{
						Metadata:   writev2.Metadata{Type: writev2.Metadata_METRIC_TYPE_GAUGE, UnitRef: 3},
						LabelsRefs: []uint32{1, 2},
					},
				},
			},
			expectError: "unit ref 3 is out of bounds of symbolsTable",
		},
		{
			name: "HelpRef bigger than symbols length",
			request: &writev2.Request{
				Symbols: []string{"", "__name__", "test"},
				Timeseries: []writev2.TimeSeries{
					{
						Metadata:   writev2.Metadata{Type: writev2.Metadata_METRIC_TYPE_GAUGE, HelpRef: 3},
						LabelsRefs: []uint32{1, 2},
					},
				},
			},
			expectError: "help ref 3 is out of bounds of symbolsTable",
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
				// Since we don't define the labels otel_scope_name and otel_scope_version, the default values coming from the receiver settings will be used.
				sm1.Scope().SetName("OpenTelemetry Collector")
				sm1.Scope().SetVersion("latest")
				metrics1 := sm1.Metrics().AppendEmpty()
				metrics1.SetName("test_metric1")
				metrics1.SetUnit("")
				metrics1.SetDescription("")

				dp1 := metrics1.SetEmptyGauge().DataPoints().AppendEmpty()
				dp1.SetTimestamp(pcommon.Timestamp(1 * int64(time.Millisecond)))
				dp1.SetDoubleValue(1.0)
				dp1.Attributes().PutStr("d", "e")
				dp1.Attributes().PutStr("foo", "bar")

				dp2 := metrics1.Gauge().DataPoints().AppendEmpty()
				dp2.SetTimestamp(pcommon.Timestamp(2 * int64(time.Millisecond)))
				dp2.SetDoubleValue(2.0)
				dp2.Attributes().PutStr("d", "e")
				dp2.Attributes().PutStr("foo", "bar")

				rm2 := expected.ResourceMetrics().AppendEmpty()
				rmAttributes2 := rm2.Resource().Attributes()
				rmAttributes2.PutStr("service.name", "foo")
				rmAttributes2.PutStr("service.instance.id", "bar")

				sm2 := rm2.ScopeMetrics().AppendEmpty()
				sm2.Scope().SetName("OpenTelemetry Collector")
				sm2.Scope().SetVersion("latest")
				metrics2 := sm2.Metrics().AppendEmpty()
				metrics2.SetName("test_metric1")
				metrics2.SetUnit("")
				metrics2.SetDescription("")

				dp3 := metrics2.SetEmptyGauge().DataPoints().AppendEmpty()
				dp3.SetTimestamp(pcommon.Timestamp(2 * int64(time.Millisecond)))
				dp3.SetDoubleValue(2.0)
				dp3.Attributes().PutStr("d", "e")
				dp3.Attributes().PutStr("foo", "bar")

				return expected
			}(),
			expectedStats: remote.WriteResponseStats{},
		},
		{
			name: "timeseries with different scopes",
			request: &writev2.Request{
				Symbols: []string{
					"",
					"__name__", "test_metric", // 1, 2
					"job", "service-x/test", // 3, 4
					"instance", "107cn001", // 5, 6
					"otel_scope_name", "scope1", // 7, 8
					"otel_scope_version", "v1", // 9, 10
					"otel_scope_name", "scope2", // 11, 12
					"otel_scope_version", "v2", // 13, 14
					"d", "e", // 15, 16
					"foo", "bar", // 17, 18
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
				metrics1 := sm1.Metrics().AppendEmpty()
				metrics1.SetName("test_metric")
				metrics1.SetUnit("")
				metrics1.SetDescription("")

				dp1 := metrics1.SetEmptyGauge().DataPoints().AppendEmpty()
				dp1.SetTimestamp(pcommon.Timestamp(1 * int64(time.Millisecond)))
				dp1.SetDoubleValue(1.0)
				dp1.Attributes().PutStr("d", "e")

				dp2 := metrics1.Gauge().DataPoints().AppendEmpty()
				dp2.SetTimestamp(pcommon.Timestamp(2 * int64(time.Millisecond)))
				dp2.SetDoubleValue(2.0)
				dp2.Attributes().PutStr("d", "e")

				sm2 := rm1.ScopeMetrics().AppendEmpty()
				sm2.Scope().SetName("scope2")
				sm2.Scope().SetVersion("v2")
				metrics2 := sm2.Metrics().AppendEmpty()
				metrics2.SetName("test_metric")
				metrics2.SetUnit("")
				metrics1.SetDescription("")

				dp3 := metrics2.SetEmptyGauge().DataPoints().AppendEmpty()
				dp3.SetTimestamp(pcommon.Timestamp(3 * int64(time.Millisecond)))
				dp3.SetDoubleValue(3.0)
				dp3.Attributes().PutStr("foo", "bar")

				return expected
			}(),
			expectedStats: remote.WriteResponseStats{},
		},
		{
			name: "separate timeseries - same labels - should be same datapointslice",
			request: &writev2.Request{
				Symbols: []string{
					"",
					"__name__", "test_metric", // 1, 2
					"job", "service-x/test", // 3, 4
					"instance", "107cn001", // 5, 6
					"otel_scope_name", "scope1", // 7, 8
					"otel_scope_version", "v1", // 9, 10
					"d", "e", // 11, 12
					"foo", "bar", // 13, 14
					"f", "g", // 15, 16
					"seconds", "milliseconds", // 17, 18
					"small desc", "longer description", // 19, 20
				},
				Timeseries: []writev2.TimeSeries{
					// The only difference between ts 0 and 1 is the value assigned in the HelpRef. According to the spec
					// Ref: https://opentelemetry.io/docs/specs/otel/metrics/data-model/#opentelemetry-protocol-data-model,
					// the HelpRef(description) field is not considered an identifying property.
					// This means that if you have two metrics with the same name, unit, scope, and resource attributes but different description values, they are still considered to be the same
					// But, between them, the longer description should be used.
					{
						Metadata:   writev2.Metadata{Type: writev2.Metadata_METRIC_TYPE_GAUGE, UnitRef: 17, HelpRef: 19},
						LabelsRefs: []uint32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12},
						Samples:    []writev2.Sample{{Value: 1, Timestamp: 1}},
					},
					{
						Metadata:   writev2.Metadata{Type: writev2.Metadata_METRIC_TYPE_GAUGE, UnitRef: 17, HelpRef: 20},
						LabelsRefs: []uint32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12},
						Samples:    []writev2.Sample{{Value: 2, Timestamp: 2}},
					},
					{
						// Unit changed, so it should be a different metric.
						Metadata:   writev2.Metadata{Type: writev2.Metadata_METRIC_TYPE_GAUGE, UnitRef: 18, HelpRef: 19},
						LabelsRefs: []uint32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 13, 14},
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

				// Expected to have 2 metrics and 3 data points.
				// The first metric should have 2 data points.
				// The second metric should have 1 data point.
				metrics1 := sm1.Metrics().AppendEmpty()
				metrics1.SetName("test_metric")
				metrics1.SetUnit("seconds")
				metrics1.SetDescription("longer description")

				dp1 := metrics1.SetEmptyGauge().DataPoints().AppendEmpty()
				dp1.SetTimestamp(pcommon.Timestamp(1 * int64(time.Millisecond)))
				dp1.SetDoubleValue(1.0)
				dp1.Attributes().PutStr("d", "e")

				dp2 := metrics1.Gauge().DataPoints().AppendEmpty()
				dp2.SetTimestamp(pcommon.Timestamp(2 * int64(time.Millisecond)))
				dp2.SetDoubleValue(2.0)
				dp2.Attributes().PutStr("d", "e")

				metrics2 := sm1.Metrics().AppendEmpty()
				metrics2.SetName("test_metric")
				metrics2.SetUnit("milliseconds")
				metrics2.SetDescription("small desc")

				dp3 := metrics2.SetEmptyGauge().DataPoints().AppendEmpty()
				dp3.SetTimestamp(pcommon.Timestamp(3 * int64(time.Millisecond)))
				dp3.SetDoubleValue(3.0)
				dp3.Attributes().PutStr("foo", "bar")

				return expected
			}(),
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
