// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewritereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusremotewritereceiver"

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	promconfig "github.com/prometheus/prometheus/config"
	writev2 "github.com/prometheus/prometheus/prompb/io/prometheus/write/v2"
	"github.com/prometheus/prometheus/storage/remote"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusremotewritereceiver/internal/metadata"
)

var writeV2RequestFixture = &writev2.Request{
	Symbols: []string{"", "__name__", "test_metric1", "job", "service-x/test", "instance", "107cn001", "d", "e", "foo", "bar", "f", "g", "h", "i", "Test gauge for test purposes", "Maybe op/sec who knows (:", "Test counter for test purposes"},
	Timeseries: []writev2.TimeSeries{
		{
			Metadata:         writev2.Metadata{Type: writev2.Metadata_METRIC_TYPE_GAUGE},
			LabelsRefs:       []uint32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, // Symbolized writeRequestFixture.Timeseries[0].Labels
			Samples:          []writev2.Sample{{Value: 1, Timestamp: 1}},
			CreatedTimestamp: 1,
		},
		{
			Metadata:         writev2.Metadata{Type: writev2.Metadata_METRIC_TYPE_GAUGE},
			LabelsRefs:       []uint32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, // Same series as first. Should use the same resource metrics.
			Samples:          []writev2.Sample{{Value: 2, Timestamp: 2}},
			CreatedTimestamp: 2,
		},
		{
			Metadata:         writev2.Metadata{Type: writev2.Metadata_METRIC_TYPE_GAUGE},
			LabelsRefs:       []uint32{1, 2, 3, 9, 5, 10, 7, 8, 9, 10}, // This series has different label values for job and instance.
			Samples:          []writev2.Sample{{Value: 2, Timestamp: 2}},
			CreatedTimestamp: 2,
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

	receiverID := component.MustNewID("test")
	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
		ReceiverID:             receiverID,
		Transport:              "http",
		ReceiverCreateSettings: receivertest.NewNopSettings(metadata.Type),
	})
	assert.NoError(t, err)

	prwReceiver.(*prometheusRemoteWriteReceiver).obsrecv = obsrecv
	writeReceiver := prwReceiver.(*prometheusRemoteWriteReceiver)

	// Add cleanup to ensure LRU cache is properly purged
	t.Cleanup(func() {
		writeReceiver.rmCache.Purge()
	})

	return writeReceiver
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
				dp1.SetStartTimestamp(pcommon.Timestamp(1 * int64(time.Millisecond)))

				dp2 := metrics1.Gauge().DataPoints().AppendEmpty()
				dp2.SetTimestamp(pcommon.Timestamp(2 * int64(time.Millisecond)))
				dp2.SetDoubleValue(2.0)
				dp2.Attributes().PutStr("d", "e")
				dp2.Attributes().PutStr("foo", "bar")
				dp2.SetStartTimestamp(pcommon.Timestamp(2 * int64(time.Millisecond)))

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
				dp3.SetStartTimestamp(pcommon.Timestamp(2 * int64(time.Millisecond)))

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
		{
			name: "service with target_info metric",
			request: &writev2.Request{
				Symbols: []string{
					"",
					"job", "production/service_a", // 1, 2
					"instance", "host1", // 3, 4
					"machine_type", "n1-standard-1", // 5, 6
					"cloud_provider", "gcp", // 7, 8
					"region", "us-central1", // 9, 10
					"datacenter", "sdc", // 11, 12
					"__name__", "normal_metric", // 13, 14
					"d", "e", // 15, 16
					"__name__", "target_info", // 17, 18
				},
				Timeseries: []writev2.TimeSeries{
					// Generating 2 metrics, one have the target_info in the name and the other is a normal gauge.
					// The target_info metric should be translated to use the resource attributes.
					// The normal_metric should be translated as usual.
					{
						// target_info metric
						Metadata:   writev2.Metadata{Type: writev2.Metadata_METRIC_TYPE_GAUGE},
						LabelsRefs: []uint32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 17, 18},
					},
					{
						// normal metric
						Metadata:   writev2.Metadata{Type: writev2.Metadata_METRIC_TYPE_GAUGE},
						LabelsRefs: []uint32{13, 14, 1, 2, 3, 4, 15, 16},
						Samples:    []writev2.Sample{{Value: 1, Timestamp: 1}},
					},
				},
			},
			expectedMetrics: func() pmetric.Metrics {
				metrics := pmetric.NewMetrics()

				rm := metrics.ResourceMetrics().AppendEmpty()
				attrs := rm.Resource().Attributes()
				attrs.PutStr("service.namespace", "production")
				attrs.PutStr("service.name", "service_a")
				attrs.PutStr("service.instance.id", "host1")
				attrs.PutStr("machine_type", "n1-standard-1")
				attrs.PutStr("cloud_provider", "gcp")
				attrs.PutStr("region", "us-central1")
				attrs.PutStr("datacenter", "sdc")

				sm := rm.ScopeMetrics().AppendEmpty()
				sm.Scope().SetName("OpenTelemetry Collector")
				sm.Scope().SetVersion("latest")

				m := sm.Metrics().AppendEmpty()
				m.SetName("normal_metric")
				m.SetUnit("")
				m.SetDescription("")

				dp := m.SetEmptyGauge().DataPoints().AppendEmpty()
				dp.SetDoubleValue(1.0)
				dp.SetTimestamp(pcommon.Timestamp(1 * int64(time.Millisecond)))
				dp.Attributes().PutStr("d", "e")

				return metrics
			}(),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// since we are using the rmCache to store values across requests, we need to clear it after each test, otherwise it will affect the next test
			prwReceiver.rmCache.Purge()
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

type nonMutatingConsumer struct{}

// Capabilities returns the base consumer capabilities.
func (bc nonMutatingConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

type MockConsumer struct {
	nonMutatingConsumer
	mu         sync.Mutex
	metrics    []pmetric.Metrics
	dataPoints int
}

func (m *MockConsumer) ConsumeMetrics(_ context.Context, md pmetric.Metrics) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.metrics = append(m.metrics, md)
	m.dataPoints += md.DataPointCount()
	return nil
}

func TestTargetInfoWithMultipleRequests(t *testing.T) {
	tests := []struct {
		name     string
		requests []*writev2.Request
	}{
		{
			name: "target_info first, normal metric second",
			requests: []*writev2.Request{
				{
					Symbols: []string{
						"",
						"job", "production/service_a", // 1, 2
						"instance", "host1", // 3, 4
						"machine_type", "n1-standard-1", // 5, 6
						"cloud_provider", "gcp", // 7, 8
						"region", "us-central1", // 9, 10
						"__name__", "target_info", // 11, 12
					},
					Timeseries: []writev2.TimeSeries{
						{
							Metadata:   writev2.Metadata{Type: writev2.Metadata_METRIC_TYPE_GAUGE},
							LabelsRefs: []uint32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12},
						},
					},
				},
				{
					Symbols: []string{
						"",
						"job", "production/service_a", // 1, 2
						"instance", "host1", // 3, 4
						"__name__", "normal_metric", // 5, 6
						"foo", "bar", // 7, 8
					},
					Timeseries: []writev2.TimeSeries{
						{
							Metadata:   writev2.Metadata{Type: writev2.Metadata_METRIC_TYPE_GAUGE},
							LabelsRefs: []uint32{5, 6, 1, 2, 3, 4, 7, 8},
							Samples:    []writev2.Sample{{Value: 2, Timestamp: 2}},
						},
					},
				},
			},
		},
		{
			name: "normal metric first, target_info second",
			requests: []*writev2.Request{
				{
					Symbols: []string{
						"",
						"job", "production/service_a", // 1, 2
						"instance", "host1", // 3, 4
						"__name__", "normal_metric", // 5, 6
						"foo", "bar", // 7, 8
					},
					Timeseries: []writev2.TimeSeries{
						{
							Metadata:   writev2.Metadata{Type: writev2.Metadata_METRIC_TYPE_GAUGE},
							LabelsRefs: []uint32{5, 6, 1, 2, 3, 4, 7, 8},
							Samples:    []writev2.Sample{{Value: 2, Timestamp: 2}},
						},
					},
				},
				{
					Symbols: []string{
						"",
						"job", "production/service_a", // 1, 2
						"instance", "host1", // 3, 4
						"machine_type", "n1-standard-1", // 5, 6
						"cloud_provider", "gcp", // 7, 8
						"region", "us-central1", // 9, 10
						"__name__", "target_info", // 11, 12
					},
					Timeseries: []writev2.TimeSeries{
						{
							Metadata:   writev2.Metadata{Type: writev2.Metadata_METRIC_TYPE_GAUGE},
							LabelsRefs: []uint32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12},
						},
					},
				},
			},
		},
	}

	// Using the same expected metrics for both tests, because we are just checking if the order of the requests changes the result.
	expectedMetrics := func() pmetric.Metrics {
		metrics := pmetric.NewMetrics()
		rm := metrics.ResourceMetrics().AppendEmpty()
		attrs := rm.Resource().Attributes()
		attrs.PutStr("service.namespace", "production")
		attrs.PutStr("service.name", "service_a")
		attrs.PutStr("service.instance.id", "host1")
		attrs.PutStr("machine_type", "n1-standard-1")
		attrs.PutStr("cloud_provider", "gcp")
		attrs.PutStr("region", "us-central1")

		sm := rm.ScopeMetrics().AppendEmpty()
		sm.Scope().SetName("OpenTelemetry Collector")
		sm.Scope().SetVersion("latest")
		m1 := sm.Metrics().AppendEmpty()
		m1.SetName("normal_metric")
		m1.SetUnit("")
		m1.SetDescription("")
		dp1 := m1.SetEmptyGauge().DataPoints().AppendEmpty()
		dp1.SetDoubleValue(2.0)
		dp1.SetTimestamp(pcommon.Timestamp(2 * int64(time.Millisecond)))
		dp1.Attributes().PutStr("foo", "bar")

		return metrics
	}()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockConsumer := new(MockConsumer)
			prwReceiver := setupMetricsReceiver(t)
			prwReceiver.nextConsumer = mockConsumer

			ts := httptest.NewServer(http.HandlerFunc(prwReceiver.handlePRW))
			defer ts.Close()

			for _, req := range tt.requests {
				pBuf := proto.NewBuffer(nil)
				// we don't need to compress the body to use the snappy compression in the unit test
				// because the encoder is just initialized when we initialize the http server.
				// so we can just use the uncompressed body.
				err := pBuf.Marshal(req)
				assert.NoError(t, err)

				resp, err := http.Post(
					ts.URL,
					fmt.Sprintf("application/x-protobuf;proto=%s", promconfig.RemoteWriteProtoMsgV2),
					bytes.NewBuffer(pBuf.Bytes()),
				)
				assert.NoError(t, err)
				defer resp.Body.Close()

				body, err := io.ReadAll(resp.Body)
				assert.NoError(t, err)
				assert.Equal(t, http.StatusNoContent, resp.StatusCode, string(body))
			}

			assert.NoError(t, pmetrictest.CompareMetrics(expectedMetrics, mockConsumer.metrics[0]))
		})
	}
}

// TestLRUCacheResourceMetrics verifies the LRU cache behavior for resource metrics:
// 1. Caching: Metrics with same job/instance share resource attributes
// 2. Eviction: When cache is full, least recently used entries are evicted
// 3. Reuse: After eviction, same job/instance metrics create new resource attributes
func TestLRUCacheResourceMetrics(t *testing.T) {
	prwReceiver := setupMetricsReceiver(t)

	// Set a small cache size to emulate the cache eviction
	prwReceiver.rmCache.Resize(1)

	t.Cleanup(func() {
		prwReceiver.rmCache.Purge()
	})

	// Metric 1.
	targetInfoRequest := &writev2.Request{
		Symbols: []string{
			"",
			"job", "production/service_a", // 1, 2
			"instance", "host1", // 3, 4
			"machine_type", "n1-standard-1", // 5, 6
			"cloud_provider", "gcp", // 7, 8
			"region", "us-central1", // 9, 10
			"__name__", "target_info", // 11, 12
		},
		Timeseries: []writev2.TimeSeries{
			{
				Metadata:   writev2.Metadata{Type: writev2.Metadata_METRIC_TYPE_GAUGE},
				LabelsRefs: []uint32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12},
			},
		},
	}

	// Metric 1 because it has the same job/instance as the target_info metric.
	metric1 := &writev2.Request{
		Symbols: []string{
			"",
			"job", "production/service_a", // 1, 2
			"instance", "host1", // 3, 4
			"__name__", "normal_metric", // 5, 6
			"foo", "bar", // 7, 8
		},
		Timeseries: []writev2.TimeSeries{
			{
				Metadata:   writev2.Metadata{Type: writev2.Metadata_METRIC_TYPE_GAUGE},
				LabelsRefs: []uint32{1, 2, 3, 4, 5, 6, 7, 8},
				Samples:    []writev2.Sample{{Value: 1, Timestamp: 1}},
			},
		},
	}

	// Metric 2. Different job/instance than metric1/target_info.
	metric2 := &writev2.Request{
		Symbols: []string{
			"",
			"job", "production/service_b", // 1, 2
			"instance", "host2", // 3, 4
			"__name__", "different_metric", // 5, 6
			"baz", "qux", // 7, 8
		},
		Timeseries: []writev2.TimeSeries{
			{
				Metadata:   writev2.Metadata{Type: writev2.Metadata_METRIC_TYPE_GAUGE},
				LabelsRefs: []uint32{1, 2, 3, 4, 5, 6, 7, 8},
				Samples:    []writev2.Sample{{Value: 2, Timestamp: 2}},
			},
		},
	}

	// Metric 1_1. Same job/instance as metric 1. As it will be inserted after cache eviction, it should not be cached.
	// And should generate a new resource metric/metric even having the same job/instance than the metric1.
	metric1_1 := &writev2.Request{
		Symbols: []string{
			"",
			"job", "production/service_a", // 1, 2
			"instance", "host1", // 3, 4
			"__name__", "normal_metric2", // 5, 6
			"joo", "kar", // 7, 8
		},
		Timeseries: []writev2.TimeSeries{
			{
				Metadata:   writev2.Metadata{Type: writev2.Metadata_METRIC_TYPE_GAUGE},
				LabelsRefs: []uint32{1, 2, 3, 4, 5, 6, 7, 8},
				Samples:    []writev2.Sample{{Value: 11, Timestamp: 11}},
			},
		},
	}

	// This metric is the result of target_info and metric1.
	expectedMetrics1 := func() pmetric.Metrics {
		metrics := pmetric.NewMetrics()
		rm := metrics.ResourceMetrics().AppendEmpty()
		attrs := rm.Resource().Attributes()
		attrs.PutStr("service.namespace", "production")
		attrs.PutStr("service.name", "service_a")
		attrs.PutStr("service.instance.id", "host1")
		attrs.PutStr("machine_type", "n1-standard-1")
		attrs.PutStr("cloud_provider", "gcp")
		attrs.PutStr("region", "us-central1")

		sm := rm.ScopeMetrics().AppendEmpty()
		sm.Scope().SetName("OpenTelemetry Collector")
		sm.Scope().SetVersion("latest")

		m1 := sm.Metrics().AppendEmpty()
		m1.SetName("normal_metric")
		m1.SetUnit("")
		m1.SetDescription("")

		dp1 := m1.SetEmptyGauge().DataPoints().AppendEmpty()
		dp1.SetDoubleValue(1.0)
		dp1.SetTimestamp(pcommon.Timestamp(1 * int64(time.Millisecond)))
		dp1.Attributes().PutStr("foo", "bar")

		return metrics
	}()

	// Result of metric2.
	expectedMetrics2 := func() pmetric.Metrics {
		metrics := pmetric.NewMetrics()
		rm := metrics.ResourceMetrics().AppendEmpty()
		attrs := rm.Resource().Attributes()
		attrs.PutStr("service.namespace", "production")
		attrs.PutStr("service.name", "service_b")
		attrs.PutStr("service.instance.id", "host2")

		sm := rm.ScopeMetrics().AppendEmpty()
		sm.Scope().SetName("OpenTelemetry Collector")
		sm.Scope().SetVersion("latest")

		m2 := sm.Metrics().AppendEmpty()
		m2.SetName("different_metric")
		m2.SetUnit("")

		dp1 := m2.SetEmptyGauge().DataPoints().AppendEmpty()
		dp1.SetDoubleValue(2.0)
		dp1.SetTimestamp(pcommon.Timestamp(2 * int64(time.Millisecond)))
		dp1.Attributes().PutStr("baz", "qux")

		return metrics
	}()

	// Result of metric1_1.
	expectedMetrics1_1 := func() pmetric.Metrics {
		metrics := pmetric.NewMetrics()
		rm := metrics.ResourceMetrics().AppendEmpty()
		attrs := rm.Resource().Attributes()
		attrs.PutStr("service.namespace", "production")
		attrs.PutStr("service.name", "service_a")
		attrs.PutStr("service.instance.id", "host1")

		sm := rm.ScopeMetrics().AppendEmpty()
		sm.Scope().SetName("OpenTelemetry Collector")
		sm.Scope().SetVersion("latest")

		m1_1 := sm.Metrics().AppendEmpty()
		m1_1.SetName("normal_metric2")
		m1_1.SetUnit("")

		dp1_1 := m1_1.SetEmptyGauge().DataPoints().AppendEmpty()
		dp1_1.SetDoubleValue(11.0)
		dp1_1.SetTimestamp(pcommon.Timestamp(11 * int64(time.Millisecond)))
		dp1_1.Attributes().PutStr("joo", "kar")

		return metrics
	}()

	mockConsumer := new(MockConsumer)
	prwReceiver.nextConsumer = mockConsumer

	ts := httptest.NewServer(http.HandlerFunc(prwReceiver.handlePRW))
	defer ts.Close()

	for _, req := range []*writev2.Request{targetInfoRequest, metric1, metric2, metric1_1} {
		pBuf := proto.NewBuffer(nil)
		err := pBuf.Marshal(req)
		assert.NoError(t, err)

		resp, err := http.Post(
			ts.URL,
			fmt.Sprintf("application/x-protobuf;proto=%s", promconfig.RemoteWriteProtoMsgV2),
			bytes.NewBuffer(pBuf.Bytes()),
		)
		assert.NoError(t, err)

		body, err := io.ReadAll(resp.Body)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusNoContent, resp.StatusCode, string(body))
	}

	// As target_info and metric1 have the same job/instance, they generate the same end metric: mockConsumer.metrics[0].
	assert.NoError(t, pmetrictest.CompareMetrics(expectedMetrics1, mockConsumer.metrics[0]))
	// As metric2 have different job/instance, it generates a different end metric: mockConsumer.metrics[2]. At this point, the cache is full it should evict the target_info metric to store the metric2.
	assert.NoError(t, pmetrictest.CompareMetrics(expectedMetrics2, mockConsumer.metrics[2]))
	// As just have 1 slot in the cache, but the cache for metric1 was evicted, this metric1_1 should generate a new resource metric, even having the same job/instance than the metric1.
	assert.NoError(t, pmetrictest.CompareMetrics(expectedMetrics1_1, mockConsumer.metrics[3]))
}
