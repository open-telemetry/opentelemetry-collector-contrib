// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewritereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusremotewritereceiver"

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	remoteapi "github.com/prometheus/client_golang/exp/api/remote"
	"github.com/prometheus/prometheus/model/value"
	writev2 "github.com/prometheus/prometheus/prompb/io/prometheus/write/v2"
	"github.com/prometheus/prometheus/storage/remote"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receiverhelper"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics/identity"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusremotewritereceiver/internal/metadata"
)

var writeV2RequestFixture = &writev2.Request{
	Symbols: []string{"", "__name__", "test_metric1", "job", "service-x/test", "instance", "107cn001", "d", "e", "foo", "bar", "f", "g", "h", "i", "Test gauge for test purposes", "Maybe op/sec who knows (:", "Test counter for test purposes"},
	Timeseries: []writev2.TimeSeries{
		{
			Metadata:   writev2.Metadata{Type: writev2.Metadata_METRIC_TYPE_GAUGE},
			LabelsRefs: []uint32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, // Symbolized writeRequestFixture.Timeseries[0].Labels
			Samples:    []writev2.Sample{{Value: 1, Timestamp: 1, StartTimestamp: 1}},
		},
		{
			Metadata:   writev2.Metadata{Type: writev2.Metadata_METRIC_TYPE_GAUGE},
			LabelsRefs: []uint32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, // Same series as first. Should use the same resource metrics.
			Samples:    []writev2.Sample{{Value: 2, Timestamp: 2, StartTimestamp: 2}},
		},
		{
			Metadata:   writev2.Metadata{Type: writev2.Metadata_METRIC_TYPE_GAUGE},
			LabelsRefs: []uint32{1, 2, 3, 9, 5, 10, 7, 8, 9, 10}, // This series has different label values for job and instance.
			Samples:    []writev2.Sample{{Value: 2, Timestamp: 2, StartTimestamp: 2}},
		},
	},
}

func setupMetricsReceiver(tb testing.TB) *prometheusRemoteWriteReceiver {
	tb.Helper()

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	prwReceiver, err := factory.CreateMetrics(tb.Context(), receivertest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	assert.NoError(tb, err)
	assert.NotNil(tb, prwReceiver, "metrics receiver creation failed")

	receiverID := component.MustNewID("test")
	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
		ReceiverID:             receiverID,
		Transport:              "http",
		ReceiverCreateSettings: receivertest.NewNopSettings(metadata.Type),
	})
	assert.NoError(tb, err)

	prwReceiver.(*prometheusRemoteWriteReceiver).obsrecv = obsrecv
	writeReceiver := prwReceiver.(*prometheusRemoteWriteReceiver)

	// Add cleanup to ensure LRU cache is properly purged
	tb.Cleanup(func() {
		writeReceiver.rmCache.Purge()
	})

	return writeReceiver
}

func TestHandlePRWContentTypeNegotiation(t *testing.T) {
	for _, tc := range []struct {
		name          string
		contentType   string
		expectedCode  int
		expectedStats remote.WriteResponseStats
	}{
		{
			name:         "no content type",
			contentType:  "",
			expectedCode: http.StatusUnsupportedMediaType,
			expectedStats: remote.WriteResponseStats{
				Confirmed:  false,
				Samples:    0,
				Histograms: 0,
				Exemplars:  0,
			},
		},
		{
			name:         "unsupported content type",
			contentType:  "application/json",
			expectedCode: http.StatusUnsupportedMediaType,
			expectedStats: remote.WriteResponseStats{
				Confirmed:  false,
				Samples:    0,
				Histograms: 0,
				Exemplars:  0,
			},
		},
		{
			name:         "x-protobuf/no proto parameter",
			contentType:  "application/x-protobuf",
			expectedCode: http.StatusUnsupportedMediaType,
			expectedStats: remote.WriteResponseStats{
				Confirmed:  false,
				Samples:    0,
				Histograms: 0,
				Exemplars:  0,
			},
		},
		{
			name:         "x-protobuf/v1 proto parameter",
			contentType:  fmt.Sprintf("application/x-protobuf;proto=%s", remoteapi.WriteV1MessageType),
			expectedCode: http.StatusUnsupportedMediaType,
			expectedStats: remote.WriteResponseStats{
				Confirmed:  false,
				Samples:    0,
				Histograms: 0,
				Exemplars:  0,
			},
		},
		{
			name:         "x-protobuf/v2 proto parameter",
			contentType:  fmt.Sprintf("application/x-protobuf;proto=%s", remoteapi.WriteV2MessageType),
			expectedCode: http.StatusNoContent,
			expectedStats: remote.WriteResponseStats{
				Confirmed:  true,
				Samples:    0,
				Histograms: 0,
				Exemplars:  0,
			},
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
	ctx, cancel := context.WithCancel(t.Context())
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
			expectedStats: remote.WriteResponseStats{
				Confirmed:  false,
				Samples:    0,
				Histograms: 0,
				Exemplars:  0,
			},
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
			expectedStats: remote.WriteResponseStats{
				Confirmed:  false,
				Samples:    0,
				Histograms: 0,
				Exemplars:  0,
			},
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
			name: "accept unspecified metric type as gauge",
			request: &writev2.Request{
				Symbols: []string{"", "__name__", "test_metric", "job", "test_job", "instance", "test_instance"},
				Timeseries: []writev2.TimeSeries{
					{
						Metadata:   writev2.Metadata{Type: writev2.Metadata_METRIC_TYPE_UNSPECIFIED},
						LabelsRefs: []uint32{1, 2, 3, 4, 5, 6},
						Samples:    []writev2.Sample{{Value: 1, Timestamp: 1, StartTimestamp: 1}},
					},
				},
			},
			expectedMetrics: func() pmetric.Metrics {
				expected := pmetric.NewMetrics()
				rm := expected.ResourceMetrics().AppendEmpty()
				attrs := rm.Resource().Attributes()
				attrs.PutStr("service.name", "test_job")
				attrs.PutStr("service.instance.id", "test_instance")

				sm := rm.ScopeMetrics().AppendEmpty()
				sm.Scope().SetName("OpenTelemetry Collector")
				sm.Scope().SetVersion("latest")
				metric := sm.Metrics().AppendEmpty()
				metric.SetName("test_metric")
				metric.SetUnit("")
				metric.SetDescription("")
				metric.Metadata().PutStr(prometheus.MetricMetadataTypeKey, "unknown")

				dp := metric.SetEmptyGauge().DataPoints().AppendEmpty()
				dp.SetTimestamp(pcommon.Timestamp(1 * int64(time.Millisecond)))
				dp.SetDoubleValue(1.0)
				dp.SetStartTimestamp(pcommon.Timestamp(1 * int64(time.Millisecond)))
				return expected
			}(),
			expectedStats: remote.WriteResponseStats{
				Confirmed:  true,
				Samples:    1,
				Histograms: 0,
				Exemplars:  0,
			},
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
				metrics1.Metadata().PutStr(prometheus.MetricMetadataTypeKey, "gauge")

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
				metrics2.Metadata().PutStr(prometheus.MetricMetadataTypeKey, "gauge")

				dp3 := metrics2.SetEmptyGauge().DataPoints().AppendEmpty()
				dp3.SetTimestamp(pcommon.Timestamp(2 * int64(time.Millisecond)))
				dp3.SetDoubleValue(2.0)
				dp3.Attributes().PutStr("d", "e")
				dp3.Attributes().PutStr("foo", "bar")
				dp3.SetStartTimestamp(pcommon.Timestamp(2 * int64(time.Millisecond)))

				return expected
			}(),
			expectedStats: remote.WriteResponseStats{
				Confirmed:  true,
				Samples:    3,
				Histograms: 0,
				Exemplars:  0,
			},
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
					{
						// NHCB histogram with scope1
						Metadata:   writev2.Metadata{Type: writev2.Metadata_METRIC_TYPE_HISTOGRAM},
						LabelsRefs: []uint32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 15, 16},
						Histograms: []writev2.Histogram{
							{
								Schema:         -53,
								Count:          &writev2.Histogram_CountInt{CountInt: 4},
								Sum:            1,
								Timestamp:      1,
								CustomValues:   []float64{1.0},
								PositiveSpans:  []writev2.BucketSpan{{Offset: 0, Length: 4}},
								PositiveDeltas: []int64{1, 2},
							},
						},
					},
					{
						// Exponential histogram with scope2
						Metadata:   writev2.Metadata{Type: writev2.Metadata_METRIC_TYPE_HISTOGRAM},
						LabelsRefs: []uint32{1, 2, 3, 4, 5, 6, 11, 12, 13, 14, 17, 18},
						Histograms: []writev2.Histogram{
							{
								Schema:         0,
								Count:          &writev2.Histogram_CountInt{CountInt: 6},
								Sum:            1,
								Timestamp:      1,
								StartTimestamp: 150,
								ZeroThreshold:  1,
								ZeroCount:      &writev2.Histogram_ZeroCountInt{ZeroCountInt: 1},
								PositiveSpans: []writev2.BucketSpan{
									{Offset: 0, Length: 2},
								},
								PositiveDeltas: []int64{2, 2},
							},
						},
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
				metrics1.Metadata().PutStr(prometheus.MetricMetadataTypeKey, "gauge")

				dp1 := metrics1.SetEmptyGauge().DataPoints().AppendEmpty()
				dp1.SetTimestamp(pcommon.Timestamp(1 * int64(time.Millisecond)))
				dp1.SetDoubleValue(1.0)
				dp1.Attributes().PutStr("d", "e")

				dp2 := metrics1.Gauge().DataPoints().AppendEmpty()
				dp2.SetTimestamp(pcommon.Timestamp(2 * int64(time.Millisecond)))
				dp2.SetDoubleValue(2.0)
				dp2.Attributes().PutStr("d", "e")

				// Add NHCB histogram to scope1
				cbneMetric := sm1.Metrics().AppendEmpty()
				cbneMetric.SetName("test_metric")
				cbneMetric.SetUnit("")
				cbneMetric.SetDescription("")
				cbneMetric.Metadata().PutStr(prometheus.MetricMetadataTypeKey, "histogram")
				hist := cbneMetric.SetEmptyHistogram()
				hist.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)

				cbneDP := hist.DataPoints().AppendEmpty()
				cbneDP.SetTimestamp(pcommon.Timestamp(1 * int64(time.Millisecond)))
				cbneDP.SetCount(4)
				cbneDP.SetSum(1)
				cbneDP.ExplicitBounds().FromRaw([]float64{1.0})
				cbneDP.BucketCounts().FromRaw([]uint64{1, 3})
				cbneDP.Attributes().PutStr("d", "e")

				sm2 := rm1.ScopeMetrics().AppendEmpty()
				sm2.Scope().SetName("scope2")
				sm2.Scope().SetVersion("v2")
				metrics2 := sm2.Metrics().AppendEmpty()
				metrics2.SetName("test_metric")
				metrics2.SetUnit("")
				metrics2.SetDescription("")
				metrics2.Metadata().PutStr(prometheus.MetricMetadataTypeKey, "gauge")

				dp3 := metrics2.SetEmptyGauge().DataPoints().AppendEmpty()
				dp3.SetTimestamp(pcommon.Timestamp(3 * int64(time.Millisecond)))
				dp3.SetDoubleValue(3.0)
				dp3.Attributes().PutStr("foo", "bar")

				// Add exponential histogram to scope2
				expMetric := sm2.Metrics().AppendEmpty()
				expMetric.SetName("test_metric")
				expMetric.SetUnit("")
				expMetric.SetDescription("")
				expMetric.Metadata().PutStr(prometheus.MetricMetadataTypeKey, "histogram")
				expHist := expMetric.SetEmptyExponentialHistogram()
				expHist.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)

				expDP := expHist.DataPoints().AppendEmpty()
				expDP.SetStartTimestamp(pcommon.Timestamp(150 * int64(time.Millisecond)))
				expDP.SetTimestamp(pcommon.Timestamp(1 * int64(time.Millisecond)))
				expDP.SetCount(6)
				expDP.SetSum(1)
				expDP.SetScale(0)
				expDP.SetZeroThreshold(1)
				expDP.SetZeroCount(1)
				expDP.Positive().SetOffset(-1)
				expDP.Positive().BucketCounts().FromRaw([]uint64{2, 4})
				expDP.Attributes().PutStr("foo", "bar")

				return expected
			}(),
			expectedStats: remote.WriteResponseStats{
				Confirmed:  true,
				Samples:    3,
				Histograms: 2,
				Exemplars:  0,
			},
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
				metrics1.Metadata().PutStr(prometheus.MetricMetadataTypeKey, "gauge")

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
				metrics2.Metadata().PutStr(prometheus.MetricMetadataTypeKey, "gauge")

				dp3 := metrics2.SetEmptyGauge().DataPoints().AppendEmpty()
				dp3.SetTimestamp(pcommon.Timestamp(3 * int64(time.Millisecond)))
				dp3.SetDoubleValue(3.0)
				dp3.Attributes().PutStr("foo", "bar")

				return expected
			}(),
			expectedStats: remote.WriteResponseStats{
				Confirmed:  true,
				Samples:    3,
				Histograms: 0,
				Exemplars:  0,
			},
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
				m.Metadata().PutStr(prometheus.MetricMetadataTypeKey, "gauge")

				dp := m.SetEmptyGauge().DataPoints().AppendEmpty()
				dp.SetDoubleValue(1.0)
				dp.SetTimestamp(pcommon.Timestamp(1 * int64(time.Millisecond)))
				dp.Attributes().PutStr("d", "e")

				return metrics
			}(),
			expectedStats: remote.WriteResponseStats{
				Confirmed:  true,
				Samples:    1,
				Histograms: 0,
				Exemplars:  0,
			},
		},
		{
			name: "exponential histogram - integer",
			request: &writev2.Request{
				Symbols: []string{
					"",
					"__name__", "test_metric", // 1, 2
					"job", "service-x/test", // 3, 4
					"instance", "107cn001", // 5, 6
					"otel_scope_name", "scope1", // 7, 8
					"otel_scope_version", "v1", // 9, 10
					"attr1", "attr1", // 11, 12
				},
				Timeseries: []writev2.TimeSeries{
					{
						Metadata: writev2.Metadata{
							Type: writev2.Metadata_METRIC_TYPE_HISTOGRAM,
						},
						Histograms: []writev2.Histogram{
							{
								Count: &writev2.Histogram_CountInt{
									CountInt: 20,
								},
								Sum:            30,
								Timestamp:      1,
								StartTimestamp: 1,
								ZeroThreshold:  1,
								ZeroCount: &writev2.Histogram_ZeroCountInt{
									ZeroCountInt: 2,
								},
								Schema:         -4,
								PositiveSpans:  []writev2.BucketSpan{{Offset: 1, Length: 2}, {Offset: 3, Length: 1}},
								NegativeSpans:  []writev2.BucketSpan{{Offset: 0, Length: 1}, {Offset: 2, Length: 1}},
								PositiveDeltas: []int64{100, 244, 221},
								NegativeDeltas: []int64{1, 2},
							},
						},
						LabelsRefs: []uint32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12},
					},
				},
			},
			expectedStats: remote.WriteResponseStats{
				Confirmed:  true,
				Samples:    0,
				Histograms: 1,
				Exemplars:  0,
			},
			expectedMetrics: func() pmetric.Metrics {
				metrics := pmetric.NewMetrics()
				rm := metrics.ResourceMetrics().AppendEmpty()
				attrs := rm.Resource().Attributes()
				attrs.PutStr("service.namespace", "service-x")
				attrs.PutStr("service.name", "test")
				attrs.PutStr("service.instance.id", "107cn001")

				sm := rm.ScopeMetrics().AppendEmpty()
				sm.Scope().SetName("scope1")
				sm.Scope().SetVersion("v1")

				m := sm.Metrics().AppendEmpty()
				m.SetName("test_metric")
				m.SetUnit("")
				m.SetDescription("")
				m.Metadata().PutStr(prometheus.MetricMetadataTypeKey, "histogram")

				hist := m.SetEmptyExponentialHistogram()
				hist.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)

				dp := hist.DataPoints().AppendEmpty()
				dp.SetTimestamp(pcommon.Timestamp(1 * int64(time.Millisecond)))
				dp.SetStartTimestamp(pcommon.Timestamp(1 * int64(time.Millisecond)))
				dp.SetScale(-4)
				dp.SetSum(30)
				dp.SetCount(20)
				dp.SetZeroCount(2)
				dp.SetZeroThreshold(1)
				dp.Positive().SetOffset(0)
				dp.Positive().BucketCounts().FromRaw([]uint64{100, 344, 0, 0, 0, 565})
				dp.Negative().BucketCounts().FromRaw([]uint64{1})
				dp.Negative().SetOffset(-1)
				dp.Negative().BucketCounts().FromRaw([]uint64{1, 0, 0, 3})
				dp.Attributes().PutStr("attr1", "attr1")
				return metrics
			}(),
		},
		{
			name: "exponential histogram - integer with negative counts",
			request: &writev2.Request{
				Symbols: []string{
					"",
					"__name__", "test_metric", // 1, 2
					"job", "service-x/test", // 3, 4
					"instance", "107cn001", // 5, 6
					"otel_scope_name", "scope1", // 7, 8
					"otel_scope_version", "v1", // 9, 10
					"attr1", "attr1", // 11, 12
				},
				Timeseries: []writev2.TimeSeries{
					{
						Metadata: writev2.Metadata{
							Type: writev2.Metadata_METRIC_TYPE_HISTOGRAM,
						},
						Histograms: []writev2.Histogram{
							{
								Count: &writev2.Histogram_CountInt{
									CountInt: 20,
								},
								Sum:            30,
								Timestamp:      1,
								StartTimestamp: 1,
								ZeroThreshold:  1,
								ZeroCount: &writev2.Histogram_ZeroCountInt{
									ZeroCountInt: 2,
								},
								Schema:         -4,
								PositiveSpans:  []writev2.BucketSpan{{Offset: 1, Length: 2}, {Offset: 3, Length: 1}},
								NegativeSpans:  []writev2.BucketSpan{{Offset: 0, Length: 1}, {Offset: 2, Length: 1}},
								PositiveDeltas: []int64{100, 244, -500},
								NegativeDeltas: []int64{1, 2},
							},
						},
						LabelsRefs: []uint32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12},
					},
				},
			},
			expectedStats: remote.WriteResponseStats{
				Confirmed:  true,
				Samples:    0,
				Histograms: 0,
				Exemplars:  0,
			},
			expectedMetrics: func() pmetric.Metrics {
				metrics := pmetric.NewMetrics()
				rm := metrics.ResourceMetrics().AppendEmpty()
				attrs := rm.Resource().Attributes()
				attrs.PutStr("service.namespace", "service-x")
				attrs.PutStr("service.name", "test")
				attrs.PutStr("service.instance.id", "107cn001")

				sm := rm.ScopeMetrics().AppendEmpty()
				sm.Scope().SetName("scope1")
				sm.Scope().SetVersion("v1")

				m := sm.Metrics().AppendEmpty()
				m.SetName("test_metric")
				m.SetUnit("")
				m.SetDescription("")
				m.Metadata().PutStr(prometheus.MetricMetadataTypeKey, "histogram")

				hist := m.SetEmptyExponentialHistogram()
				hist.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)

				return metrics
			}(),
		},
		{
			name: "exponential histogram - float",
			request: &writev2.Request{
				Symbols: []string{
					"",
					"__name__", "test_metric", // 1, 2
					"job", "service-x/test", // 3, 4
					"instance", "107cn001", // 5, 6
					"otel_scope_name", "scope1", // 7, 8
					"otel_scope_version", "v1", // 9, 10
					"attr1", "attr1", // 11, 12
				},
				Timeseries: []writev2.TimeSeries{
					{
						Metadata: writev2.Metadata{
							Type: writev2.Metadata_METRIC_TYPE_HISTOGRAM,
						},
						Histograms: []writev2.Histogram{
							{
								Count: &writev2.Histogram_CountFloat{
									CountFloat: 20,
								},
								Sum:            33.3,
								Timestamp:      1,
								StartTimestamp: 1,
								ZeroThreshold:  1,
								ZeroCount: &writev2.Histogram_ZeroCountFloat{
									ZeroCountFloat: 2,
								},
								Schema:         -4,
								PositiveSpans:  []writev2.BucketSpan{{Offset: 1, Length: 2}, {Offset: 3, Length: 1}, {Offset: 5, Length: 1}},
								NegativeSpans:  []writev2.BucketSpan{{Offset: 0, Length: 1}},
								NegativeCounts: []float64{1},
								PositiveCounts: []float64{33, 30, 26, 100},
							},
						},
						LabelsRefs: []uint32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12},
					},
				},
			},
			expectedMetrics: func() pmetric.Metrics {
				metrics := pmetric.NewMetrics()
				rm := metrics.ResourceMetrics().AppendEmpty()
				attrs := rm.Resource().Attributes()
				attrs.PutStr("service.namespace", "service-x")
				attrs.PutStr("service.name", "test")
				attrs.PutStr("service.instance.id", "107cn001")

				sm := rm.ScopeMetrics().AppendEmpty()
				sm.Scope().SetName("scope1")
				sm.Scope().SetVersion("v1")

				m := sm.Metrics().AppendEmpty()
				m.SetName("test_metric")
				m.SetUnit("")
				m.Metadata().PutStr(prometheus.MetricMetadataTypeKey, "histogram")

				hist := m.SetEmptyExponentialHistogram()
				hist.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)

				dp := hist.DataPoints().AppendEmpty()
				dp.SetTimestamp(pcommon.Timestamp(1 * int64(time.Millisecond)))
				dp.SetStartTimestamp(pcommon.Timestamp(1 * int64(time.Millisecond)))
				dp.SetScale(-4)
				dp.SetSum(33.3)
				dp.SetCount(20)
				dp.SetZeroCount(2)
				dp.SetZeroThreshold(1)
				dp.Positive().SetOffset(0)
				dp.Positive().BucketCounts().FromRaw([]uint64{33, 30, 0, 0, 0, 26, 0, 0, 0, 0, 0, 100})
				dp.Negative().BucketCounts().FromRaw([]uint64{1})
				dp.Negative().SetOffset(-1)
				dp.Negative().BucketCounts().FromRaw([]uint64{1})
				dp.Attributes().PutStr("attr1", "attr1")

				return metrics
			}(),
			expectedStats: remote.WriteResponseStats{
				Confirmed:  true,
				Samples:    0,
				Histograms: 1,
				Exemplars:  0,
			},
		},
		{
			name: "exponential histogram - float with negative counts",
			request: &writev2.Request{
				Symbols: []string{
					"",
					"__name__", "test_metric", // 1, 2
					"job", "service-x/test", // 3, 4
					"instance", "107cn001", // 5, 6
					"otel_scope_name", "scope1", // 7, 8
					"otel_scope_version", "v1", // 9, 10
				},
				Timeseries: []writev2.TimeSeries{
					{
						Metadata: writev2.Metadata{
							Type: writev2.Metadata_METRIC_TYPE_HISTOGRAM,
						},
						Histograms: []writev2.Histogram{
							{
								Count: &writev2.Histogram_CountFloat{
									CountFloat: 20,
								},
								Sum:            33.3,
								Timestamp:      1,
								StartTimestamp: 1,
								ZeroThreshold:  1,
								ZeroCount: &writev2.Histogram_ZeroCountFloat{
									ZeroCountFloat: 2,
								},
								Schema:         -4,
								PositiveSpans:  []writev2.BucketSpan{{Offset: 1, Length: 2}, {Offset: 3, Length: 1}, {Offset: 5, Length: 1}},
								NegativeSpans:  []writev2.BucketSpan{{Offset: 0, Length: 1}},
								NegativeCounts: []float64{1},
								// As we are passing negative counts, the translation should drop the sample.
								PositiveCounts: []float64{-33, 30, 26, -100},
							},
						},
						LabelsRefs: []uint32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
					},
				},
			},
			expectedStats: remote.WriteResponseStats{
				Confirmed:  true,
				Samples:    0,
				Histograms: 0,
				Exemplars:  0,
			},
			expectedMetrics: func() pmetric.Metrics {
				metrics := pmetric.NewMetrics()
				rm := metrics.ResourceMetrics().AppendEmpty()
				attrs := rm.Resource().Attributes()
				attrs.PutStr("service.namespace", "service-x")
				attrs.PutStr("service.name", "test")
				attrs.PutStr("service.instance.id", "107cn001")

				sm := rm.ScopeMetrics().AppendEmpty()
				sm.Scope().SetName("scope1")
				sm.Scope().SetVersion("v1")

				m := sm.Metrics().AppendEmpty()
				m.SetName("test_metric")
				m.SetUnit("")
				m.Metadata().PutStr(prometheus.MetricMetadataTypeKey, "histogram")

				hist := m.SetEmptyExponentialHistogram()
				hist.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)

				return metrics
			}(),
		},
		{
			name: "reset hint gauge should be dropped",
			request: &writev2.Request{
				Symbols: []string{
					"",
					"__name__", "test_metric", // 1, 2
					"job", "service-x/test", // 3, 4
					"instance", "107cn001", // 5, 6
					"otel_scope_name", "scope1", // 7, 8
					"otel_scope_version", "v1", // 9, 10
				},
				Timeseries: []writev2.TimeSeries{
					{
						Metadata: writev2.Metadata{
							Type: writev2.Metadata_METRIC_TYPE_HISTOGRAM,
						},
						Histograms: []writev2.Histogram{
							{
								ResetHint: writev2.Histogram_RESET_HINT_GAUGE,
								Count: &writev2.Histogram_CountFloat{
									CountFloat: 20,
								},
								Sum:           33.3,
								Timestamp:     1,
								ZeroThreshold: 1,
								ZeroCount: &writev2.Histogram_ZeroCountFloat{
									ZeroCountFloat: 2,
								},
								Schema:         -4,
								PositiveSpans:  []writev2.BucketSpan{{Offset: 1, Length: 2}},
								NegativeSpans:  []writev2.BucketSpan{{Offset: 0, Length: 1}},
								NegativeCounts: []float64{1},
								PositiveCounts: []float64{33},
							},
						},
						LabelsRefs: []uint32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
					},
				},
			},
			expectedStats: remote.WriteResponseStats{
				Confirmed:  true,
				Samples:    0,
				Histograms: 0,
				Exemplars:  0,
			},
			expectedMetrics: pmetric.NewMetrics(), // Reset hint gauge should be dropped completely, no resources should be created
		},
		{
			name: "classic histogram - should be dropped",
			request: &writev2.Request{
				Symbols: []string{
					"",
					"__name__", "test_metric", // 1, 2
					"job", "service-x/test", // 3, 4
					"instance", "107cn001", // 5, 6
					"otel_scope_name", "scope1", // 7, 8
					"otel_scope_version", "v1", // 9, 10
					"attr1", "attr1", // 11, 12
				},
				Timeseries: []writev2.TimeSeries{
					{
						Metadata: writev2.Metadata{
							Type: writev2.Metadata_METRIC_TYPE_HISTOGRAM,
						},
						// Classic histograms populate samples instead of histograms. Those should be dropped.
						Histograms: []writev2.Histogram{},
						Samples: []writev2.Sample{
							{
								Value:     1,
								Timestamp: 1,
							},
						},
						LabelsRefs: []uint32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12},
					},
				},
			},
			expectedStats: remote.WriteResponseStats{
				Confirmed:  true,
				Samples:    0,
				Histograms: 0,
				Exemplars:  0,
			},
			expectedMetrics: pmetric.NewMetrics(), // Classic histograms should be dropped completely, no resources should be created
		},
		{
			name: "summary - should be dropped",
			request: &writev2.Request{
				Symbols: []string{
					"",
					"__name__", "test_metric", // 1, 2
					"job", "service-x/test", // 3, 4
					"instance", "107cn001", // 5, 6
					"otel_scope_name", "scope1", // 7, 8
					"otel_scope_version", "v1", // 9, 10
				},
				Timeseries: []writev2.TimeSeries{
					{
						Metadata: writev2.Metadata{
							Type: writev2.Metadata_METRIC_TYPE_SUMMARY,
						},
						Samples: []writev2.Sample{
							{
								Value:     1,
								Timestamp: 1,
							},
						},
						LabelsRefs: []uint32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
					},
				},
			},
			expectedStats: remote.WriteResponseStats{
				Confirmed:  true,
				Samples:    0,
				Histograms: 0,
				Exemplars:  0,
			},
			expectedMetrics: func() pmetric.Metrics {
				metrics := pmetric.NewMetrics()
				rm := metrics.ResourceMetrics().AppendEmpty()
				attrs := rm.Resource().Attributes()
				attrs.PutStr("service.namespace", "service-x")
				attrs.PutStr("service.name", "test")
				attrs.PutStr("service.instance.id", "107cn001")

				sm := rm.ScopeMetrics().AppendEmpty()
				sm.Scope().SetName("scope1")
				sm.Scope().SetVersion("v1")

				return metrics
			}(),
		},
		{
			name: "NHCB translation",
			request: &writev2.Request{
				Symbols: []string{
					"",
					"__name__",
					"test_hncb_histogram",
					"job",
					"test",
					"instance",
					"localhost:8080",
					"seconds",
					"Test NHCB histogram",
				},
				Timeseries: []writev2.TimeSeries{
					{
						LabelsRefs: []uint32{1, 2, 3, 4, 5, 6}, // __name__=test_hncb_histogram, job=test, instance=localhost:8080
						Metadata: writev2.Metadata{
							Type: writev2.Metadata_METRIC_TYPE_HISTOGRAM,
						},
						Histograms: []writev2.Histogram{
							{
								Timestamp:      123456789,
								StartTimestamp: 123456000,
								Schema:         -53, // NHCB schema
								Sum:            100.5,
								Count:          &writev2.Histogram_CountInt{CountInt: 180},
								CustomValues:   []float64{1.0, 2.0, 5.0, 10.0}, // Custom bucket boundaries
								PositiveSpans: []writev2.BucketSpan{
									{Offset: 0, Length: 5}, // 5 buckets: 4 custom + 1 overflow
								},
								PositiveDeltas: []int64{10, 15, 20, 5, 0}, // Delta counts for each bucket
							},
						},
					},
				},
			},
			expectedStats: remote.WriteResponseStats{
				Confirmed:  true,
				Samples:    0,
				Histograms: 1,
				Exemplars:  0,
			},
			expectedMetrics: func() pmetric.Metrics {
				metrics := pmetric.NewMetrics()
				rm := metrics.ResourceMetrics().AppendEmpty()
				attrs := rm.Resource().Attributes()
				attrs.PutStr("service.name", "test")
				attrs.PutStr("service.instance.id", "localhost:8080")

				sm := rm.ScopeMetrics().AppendEmpty()
				sm.Scope().SetName("OpenTelemetry Collector")
				sm.Scope().SetVersion("latest")
				m1 := sm.Metrics().AppendEmpty()
				m1.SetName("test_hncb_histogram")
				m1.SetUnit("")
				m1.SetDescription("")
				m1.Metadata().PutStr(prometheus.MetricMetadataTypeKey, "histogram")
				hist := m1.SetEmptyHistogram()
				hist.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)

				dp := hist.DataPoints().AppendEmpty()
				dp.SetStartTimestamp(pcommon.Timestamp(123456000 * int64(time.Millisecond)))
				dp.SetTimestamp(pcommon.Timestamp(123456789 * int64(time.Millisecond)))
				dp.SetSum(100.5)
				dp.SetCount(180)
				dp.ExplicitBounds().FromRaw([]float64{1.0, 2.0, 5.0, 10.0})
				dp.BucketCounts().FromRaw([]uint64{10, 25, 45, 50, 50})

				return metrics
			}(),
		},
		{
			name: "NHCB translation with stale NaN",
			request: &writev2.Request{
				Symbols: []string{
					"",
					"__name__",
					"test_hncb_histogram_stale",
					"job",
					"test",
					"instance",
					"localhost:8080",
					"seconds",
					"Test NHCB histogram with stale NaN",
				},
				Timeseries: []writev2.TimeSeries{
					{
						LabelsRefs: []uint32{1, 2, 3, 4, 5, 6}, // __name__=test_hncb_histogram_stale, job=test, instance=localhost:8080
						Metadata: writev2.Metadata{
							Type: writev2.Metadata_METRIC_TYPE_HISTOGRAM,
						},
						Histograms: []writev2.Histogram{
							{
								Timestamp:    123456789,
								Schema:       -53, // NHCB schema
								Sum:          math.Float64frombits(value.StaleNaN),
								Count:        &writev2.Histogram_CountInt{CountInt: 0},
								CustomValues: []float64{1.0, 2.0, 5.0, 10.0}, // Custom bucket boundaries
							},
						},
					},
				},
			},
			expectedStats: remote.WriteResponseStats{
				Confirmed:  true,
				Samples:    0,
				Histograms: 1,
				Exemplars:  0,
			},
			expectedMetrics: func() pmetric.Metrics {
				metrics := pmetric.NewMetrics()
				rm := metrics.ResourceMetrics().AppendEmpty()
				attrs := rm.Resource().Attributes()
				attrs.PutStr("service.name", "test")
				attrs.PutStr("service.instance.id", "localhost:8080")

				sm := rm.ScopeMetrics().AppendEmpty()
				sm.Scope().SetName("OpenTelemetry Collector")
				sm.Scope().SetVersion("latest")
				m1 := sm.Metrics().AppendEmpty()
				m1.SetName("test_hncb_histogram_stale")
				m1.SetUnit("")
				m1.SetDescription("")
				m1.Metadata().PutStr(prometheus.MetricMetadataTypeKey, "histogram")
				hist := m1.SetEmptyHistogram()
				hist.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)

				dp := hist.DataPoints().AppendEmpty()
				dp.SetTimestamp(pcommon.Timestamp(123456789 * int64(time.Millisecond)))
				dp.SetFlags(pmetric.DefaultDataPointFlags.WithNoRecordedValue(true))
				dp.ExplicitBounds().FromRaw([]float64{1.0, 2.0, 5.0, 10.0})
				dp.BucketCounts().FromRaw([]uint64{0, 0, 0, 0, 0})

				return metrics
			}(),
		},
		{
			name: "invalid schema histogram dropped",
			request: &writev2.Request{
				Symbols: []string{
					"",
					"__name__",
					"test_invalid_schema_histogram",
					"job",
					"test",
					"instance",
					"localhost:8080",
				},
				Timeseries: []writev2.TimeSeries{
					{
						LabelsRefs: []uint32{1, 2, 3, 4, 5, 6}, // __name__=test_invalid_schema_histogram, job=test, instance=localhost:8080
						Metadata: writev2.Metadata{
							Type: writev2.Metadata_METRIC_TYPE_HISTOGRAM,
						},
						Histograms: []writev2.Histogram{
							{
								Timestamp: 123456789,
								Schema:    -10, // Invalid schema (outside -4 to 8 range)
								Sum:       100.5,
								Count:     &writev2.Histogram_CountInt{CountInt: 50},
								PositiveSpans: []writev2.BucketSpan{
									{Offset: 0, Length: 2},
								},
								PositiveDeltas: []int64{10, 15},
							},
							{
								Timestamp: 123456789,
								Schema:    15, // Invalid schema (outside -4 to 8 range)
								Sum:       200.5,
								Count:     &writev2.Histogram_CountInt{CountInt: 100},
								PositiveSpans: []writev2.BucketSpan{
									{Offset: 0, Length: 2},
								},
								PositiveDeltas: []int64{20, 25},
							},
						},
					},
				},
			},
			expectedStats: remote.WriteResponseStats{
				Confirmed:  true,
				Samples:    0,
				Histograms: 0,
				Exemplars:  0,
			},
			expectedMetrics: pmetric.NewMetrics(), // When all histograms have invalid schemas, no metrics should be created
		},
		{
			name: "mixed schema histograms - NHCB and exponential",
			request: &writev2.Request{
				Symbols: []string{
					"",
					"__name__",
					"test_mixed_histogram",
					"job",
					"test",
					"instance",
					"localhost:8080",
					"seconds",
					"Test mixed histogram",
				},
				Timeseries: []writev2.TimeSeries{
					{
						LabelsRefs: []uint32{1, 2, 3, 4, 5, 6}, // __name__=test_mixed_histogram, job=test, instance=localhost:8080
						Metadata: writev2.Metadata{
							Type: writev2.Metadata_METRIC_TYPE_HISTOGRAM,
						},
						Histograms: []writev2.Histogram{
							{
								// First histogram - NHCB
								Timestamp:      123456789,
								StartTimestamp: 123456000,
								Schema:         -53,
								Sum:            100.5,
								Count:          &writev2.Histogram_CountInt{CountInt: 180},
								CustomValues:   []float64{1.0, 2.0, 5.0, 10.0},
								PositiveSpans: []writev2.BucketSpan{
									{Offset: 0, Length: 5},
								},
								PositiveDeltas: []int64{10, 15, 20, 5, 0},
							},
							{
								// Second histogram - Exponential
								Timestamp:      123456790,
								StartTimestamp: 123456000,
								Schema:         -4,
								Sum:            200.0,
								Count:          &writev2.Histogram_CountInt{CountInt: 100},
								ZeroThreshold:  1.0,
								ZeroCount:      &writev2.Histogram_ZeroCountInt{ZeroCountInt: 5},
								PositiveSpans: []writev2.BucketSpan{
									{Offset: 1, Length: 2},
								},
								PositiveDeltas: []int64{30, 40},
							},
						},
					},
				},
			},
			expectedStats: remote.WriteResponseStats{
				Confirmed:  true,
				Samples:    0,
				Histograms: 2,
				Exemplars:  0,
			},
			expectedMetrics: func() pmetric.Metrics {
				metrics := pmetric.NewMetrics()
				rm := metrics.ResourceMetrics().AppendEmpty()
				attrs := rm.Resource().Attributes()
				attrs.PutStr("service.name", "test")
				attrs.PutStr("service.instance.id", "localhost:8080")

				sm := rm.ScopeMetrics().AppendEmpty()
				sm.Scope().SetName("OpenTelemetry Collector")
				sm.Scope().SetVersion("latest")

				// First metric - NHCB (regular histogram)
				m1 := sm.Metrics().AppendEmpty()
				m1.SetName("test_mixed_histogram")
				m1.SetUnit("")
				m1.SetDescription("")
				hist1 := m1.SetEmptyHistogram()
				hist1.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)

				dp1 := hist1.DataPoints().AppendEmpty()
				dp1.SetStartTimestamp(pcommon.Timestamp(123456000 * int64(time.Millisecond)))
				dp1.SetTimestamp(pcommon.Timestamp(123456789 * int64(time.Millisecond)))
				dp1.SetSum(100.5)
				dp1.SetCount(180)
				dp1.ExplicitBounds().FromRaw([]float64{1.0, 2.0, 5.0, 10.0})
				dp1.BucketCounts().FromRaw([]uint64{10, 25, 45, 50, 50})

				// Second metric - Exponential histogram
				m2 := sm.Metrics().AppendEmpty()
				m2.SetName("test_mixed_histogram")
				m2.SetUnit("")
				m2.SetDescription("")
				m2.Metadata().PutStr(prometheus.MetricMetadataTypeKey, "histogram")
				hist2 := m2.SetEmptyExponentialHistogram()
				hist2.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)

				dp2 := hist2.DataPoints().AppendEmpty()
				dp2.SetStartTimestamp(pcommon.Timestamp(123456000 * int64(time.Millisecond)))
				dp2.SetTimestamp(pcommon.Timestamp(123456790 * int64(time.Millisecond)))
				dp2.SetScale(-4)
				dp2.SetSum(200.0)
				dp2.SetCount(100)
				dp2.SetZeroCount(5)
				dp2.SetZeroThreshold(1.0)
				dp2.Positive().SetOffset(0)
				dp2.Positive().BucketCounts().FromRaw([]uint64{30, 70})

				return metrics
			}(),
		},
		{
			name: "unspecified histogram",
			request: &writev2.Request{
				Symbols: []string{
					"",
					"__name__", "test_metric", // 1, 2
					"job", "service-x/test", // 3, 4
					"instance", "107cn001", // 5, 6
					"otel_scope_name", "scope1", // 7, 8
					"otel_scope_version", "v1", // 9, 10
					"attr1", "attr1", // 11, 12
				},
				Timeseries: []writev2.TimeSeries{
					{
						Metadata: writev2.Metadata{},
						Histograms: []writev2.Histogram{
							{
								Count: &writev2.Histogram_CountInt{
									CountInt: 20,
								},
								Sum:            30,
								Timestamp:      1,
								StartTimestamp: 1,
								ZeroThreshold:  1,
								ZeroCount: &writev2.Histogram_ZeroCountInt{
									ZeroCountInt: 2,
								},
								Schema:         -4,
								PositiveSpans:  []writev2.BucketSpan{{Offset: 1, Length: 2}, {Offset: 3, Length: 1}},
								NegativeSpans:  []writev2.BucketSpan{{Offset: 0, Length: 1}, {Offset: 2, Length: 1}},
								PositiveDeltas: []int64{100, 244, 221},
								NegativeDeltas: []int64{1, 2},
							},
						},
						LabelsRefs: []uint32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12},
					},
				},
			},
			expectedStats: remote.WriteResponseStats{
				Confirmed:  true,
				Samples:    0,
				Histograms: 1,
				Exemplars:  0,
			},
			expectedMetrics: func() pmetric.Metrics {
				metrics := pmetric.NewMetrics()
				rm := metrics.ResourceMetrics().AppendEmpty()
				attrs := rm.Resource().Attributes()
				attrs.PutStr("service.namespace", "service-x")
				attrs.PutStr("service.name", "test")
				attrs.PutStr("service.instance.id", "107cn001")

				sm := rm.ScopeMetrics().AppendEmpty()
				sm.Scope().SetName("scope1")
				sm.Scope().SetVersion("v1")

				m := sm.Metrics().AppendEmpty()
				m.SetName("test_metric")
				m.SetUnit("")
				m.SetDescription("")
				m.Metadata().PutStr(prometheus.MetricMetadataTypeKey, "unknown")

				hist := m.SetEmptyExponentialHistogram()
				hist.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)

				dp := hist.DataPoints().AppendEmpty()
				dp.SetTimestamp(pcommon.Timestamp(1 * int64(time.Millisecond)))
				dp.SetStartTimestamp(pcommon.Timestamp(1 * int64(time.Millisecond)))
				dp.SetScale(-4)
				dp.SetSum(30)
				dp.SetCount(20)
				dp.SetZeroCount(2)
				dp.SetZeroThreshold(1)
				dp.Positive().SetOffset(0)
				dp.Positive().BucketCounts().FromRaw([]uint64{100, 344, 0, 0, 0, 565})
				dp.Negative().BucketCounts().FromRaw([]uint64{1})
				dp.Negative().SetOffset(-1)
				dp.Negative().BucketCounts().FromRaw([]uint64{1, 0, 0, 3})
				dp.Attributes().PutStr("attr1", "attr1")
				return metrics
			}(),
		},
		{
			name: "unspecified nhcb",
			request: &writev2.Request{
				Symbols: []string{
					"",
					"__name__", "test_hncb_histogram", // 1,2
					"job", "test", // 3, 4
					"instance", "localhost:8080", // 5, 6
					"seconds", "Test NHCB histogram", // 7, 8
				},
				Timeseries: []writev2.TimeSeries{
					{
						LabelsRefs: []uint32{1, 2, 3, 4, 5, 6},
						Metadata:   writev2.Metadata{},
						Histograms: []writev2.Histogram{
							{
								Timestamp:      123456789,
								StartTimestamp: 123456000,
								Schema:         -53, // NHCB schema
								Sum:            100.5,
								Count:          &writev2.Histogram_CountInt{CountInt: 180},
								CustomValues:   []float64{1.0, 2.0, 5.0, 10.0}, // Custom bucket boundaries
								PositiveSpans: []writev2.BucketSpan{
									{Offset: 0, Length: 5}, // 5 buckets: 4 custom + 1 overflow
								},
								PositiveDeltas: []int64{10, 15, 20, 5, 0}, // Delta counts for each bucket
							},
						},
					},
				},
			},
			expectedStats: remote.WriteResponseStats{
				Confirmed:  true,
				Samples:    0,
				Histograms: 1,
				Exemplars:  0,
			},
			expectedMetrics: func() pmetric.Metrics {
				metrics := pmetric.NewMetrics()
				rm := metrics.ResourceMetrics().AppendEmpty()
				attrs := rm.Resource().Attributes()
				attrs.PutStr("service.name", "test")
				attrs.PutStr("service.instance.id", "localhost:8080")

				sm := rm.ScopeMetrics().AppendEmpty()
				sm.Scope().SetName("OpenTelemetry Collector")
				sm.Scope().SetVersion("latest")
				m1 := sm.Metrics().AppendEmpty()
				m1.SetName("test_hncb_histogram")
				m1.SetUnit("")
				m1.SetDescription("")
				m1.Metadata().PutStr(prometheus.MetricMetadataTypeKey, "unknown")
				hist := m1.SetEmptyHistogram()
				hist.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)

				dp := hist.DataPoints().AppendEmpty()
				dp.SetStartTimestamp(pcommon.Timestamp(123456000 * int64(time.Millisecond)))
				dp.SetTimestamp(pcommon.Timestamp(123456789 * int64(time.Millisecond)))
				dp.SetSum(100.5)
				dp.SetCount(180)
				dp.ExplicitBounds().FromRaw([]float64{1.0, 2.0, 5.0, 10.0})
				dp.BucketCounts().FromRaw([]uint64{10, 25, 45, 50, 50})

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
			assert.Equal(t, buildMetaDataMapByID(tc.expectedMetrics), buildMetaDataMapByID(metrics))
		})
	}
}

type nonMutatingConsumer struct{}

// Capabilities returns the base consumer capabilities.
func (nonMutatingConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

type mockConsumer struct {
	nonMutatingConsumer
	mu         sync.Mutex
	metrics    []pmetric.Metrics
	dataPoints int
}

func (m *mockConsumer) ConsumeMetrics(_ context.Context, md pmetric.Metrics) error {
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
	}

	expectedIndex0 := func() pmetric.Metrics {
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
			mockConsumer := new(mockConsumer)
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
					fmt.Sprintf("application/x-protobuf;proto=%s", remoteapi.WriteV2MessageType),
					bytes.NewBuffer(pBuf.Bytes()),
				)
				assert.NoError(t, err)
				defer resp.Body.Close()

				body, err := io.ReadAll(resp.Body)
				assert.NoError(t, err)
				assert.Equal(t, http.StatusNoContent, resp.StatusCode, string(body))
			}
			// Only one metric should be emitted
			assert.Len(t, mockConsumer.metrics, 1)
			// index 0 is related to the join between the target_info and the normal metric.
			assert.NoError(t, pmetrictest.CompareMetrics(expectedIndex0, mockConsumer.metrics[0]))
		})
	}
}

func TestRemoteWriteHistogramWithExemplars(t *testing.T) {
	tests := []struct {
		name     string
		requests []*writev2.Request
		expected func() pmetric.Metrics
	}{
		{
			name: "histogram metric with exemplar",
			requests: []*writev2.Request{
				{
					Symbols: []string{
						"",
						"job", "production/service_a", // 1,2
						"instance", "host1", // 3,4
						"__name__", "request_duration_ms", // 5,6
						"trace_id", "4bf92f3577b34da6a3ce929d0e0e4736", // 7,8
						"span_id", "00f067aa0ba902b7", // 9,10,
						"4bf92f3577b34da6a3ce929d0e0e4740", "fff067aa0ba902b7", // 11, 12
					},
					Timeseries: []writev2.TimeSeries{
						{
							Metadata: writev2.Metadata{
								Type: writev2.Metadata_METRIC_TYPE_HISTOGRAM,
							},
							LabelsRefs: []uint32{
								5, 6, // __name__
								1, 2, // job
								3, 4, // instance
							},
							Histograms: []writev2.Histogram{
								{
									Schema:         -53,
									Count:          &writev2.Histogram_CountInt{CountInt: 4},
									Sum:            1,
									Timestamp:      1,
									CustomValues:   []float64{1.0},
									PositiveSpans:  []writev2.BucketSpan{{Offset: 0, Length: 4}},
									PositiveDeltas: []int64{1, 2},
								},
							},
						},
						// We're sending exemplars disconnected from histograms because
						// remote-write 2.0 emits exemplars independently of histogram samples.
						// See https://github.com/prometheus/prometheus/issues/17857.
						{
							Metadata: writev2.Metadata{
								Type: writev2.Metadata_METRIC_TYPE_HISTOGRAM,
							},
							LabelsRefs: []uint32{
								5, 6, // __name__
								1, 2, // job
								3, 4, // instance
							},
							Exemplars: []writev2.Exemplar{
								{
									Value:     1.0,
									Timestamp: 1,
									LabelsRefs: []uint32{
										7, 8, // trace_id
										9, 10, // span_id
									},
								},
							},
						},
					},
				},
			},
			expected: func() pmetric.Metrics {
				metrics := pmetric.NewMetrics()
				rm := metrics.ResourceMetrics().AppendEmpty()
				attrs := rm.Resource().Attributes()
				attrs.PutStr("service.namespace", "production")
				attrs.PutStr("service.name", "service_a")
				attrs.PutStr("service.instance.id", "host1")

				sm := rm.ScopeMetrics().AppendEmpty()
				sm.Scope().SetName("OpenTelemetry Collector")
				sm.Scope().SetVersion("latest")

				m := sm.Metrics().AppendEmpty()
				m.SetName("request_duration_ms")
				m.SetUnit("")
				m.SetDescription("")

				hist := m.SetEmptyHistogram()
				hist.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)

				dp := hist.DataPoints().AppendEmpty()
				dp.SetCount(4)
				dp.SetSum(1)
				dp.SetTimestamp(pcommon.Timestamp(1 * int64(time.Millisecond)))

				ex := dp.Exemplars().AppendEmpty()
				ex.SetDoubleValue(1.0)
				ex.SetTimestamp(pcommon.Timestamp(1 * int64(time.Millisecond)))

				traceID, _ := hex.DecodeString("4bf92f3577b34da6a3ce929d0e0e4736")
				var tid [16]byte
				copy(tid[:], traceID)
				ex.SetTraceID(pcommon.TraceID(tid))

				spanID, _ := hex.DecodeString("00f067aa0ba902b7")
				var sid [8]byte
				copy(sid[:], spanID)
				ex.SetSpanID(pcommon.SpanID(sid))

				// Bucket counts from PositiveDeltas
				dp.BucketCounts().Append(1)
				dp.BucketCounts().Append(3)
				// Custom boundaries
				dp.ExplicitBounds().Append(1.0) // matches CustomValues
				return metrics
			},
		},
		{
			name: "multiple histogram metrics with exemplar",
			requests: []*writev2.Request{
				{
					Symbols: []string{
						"",
						"job", "production/service_a", // 1,2
						"instance", "host1", // 3,4
						"__name__", "request_duration_ms", // 5,6
						"trace_id", "4bf92f3577b34da6a3ce929d0e0e4736", // 7,8
						"span_id", "00f067aa0ba902b7", // 9,10,
						"4bf92f3577b34da6a3ce929d0e0e4740", "fff067aa0ba902b7", // 11, 12
						"latency_ms", // 13
					},
					Timeseries: []writev2.TimeSeries{
						{
							Metadata: writev2.Metadata{
								Type: writev2.Metadata_METRIC_TYPE_HISTOGRAM,
							},
							LabelsRefs: []uint32{
								5, 6, // __name__
								1, 2, // job
								3, 4, // instance
							},
							Histograms: []writev2.Histogram{
								{
									Schema:         -53,
									Count:          &writev2.Histogram_CountInt{CountInt: 4},
									Sum:            1,
									Timestamp:      1,
									CustomValues:   []float64{1.0},
									PositiveSpans:  []writev2.BucketSpan{{Offset: 0, Length: 4}},
									PositiveDeltas: []int64{1, 2},
								},
							},
						},
						{
							Metadata: writev2.Metadata{
								Type: writev2.Metadata_METRIC_TYPE_HISTOGRAM,
							},
							LabelsRefs: []uint32{
								5, 13, // __name__
								1, 2, // job
								3, 4, // instance
							},
							Histograms: []writev2.Histogram{
								{
									Schema:         -53,
									Count:          &writev2.Histogram_CountInt{CountInt: 4},
									Sum:            1,
									Timestamp:      1,
									CustomValues:   []float64{1.0},
									PositiveSpans:  []writev2.BucketSpan{{Offset: 0, Length: 4}},
									PositiveDeltas: []int64{1, 2},
								},
							},
						},
						{
							Metadata: writev2.Metadata{
								Type: writev2.Metadata_METRIC_TYPE_HISTOGRAM,
							},
							LabelsRefs: []uint32{
								5, 6, // __name__
								1, 2, // job
								3, 4, // instance
							},
							Exemplars: []writev2.Exemplar{
								{
									Value:     1.0,
									Timestamp: 1,
									LabelsRefs: []uint32{
										7, 8, // trace_id
										9, 10, // span_id
									},
								},
							},
						},
						{
							Metadata: writev2.Metadata{
								Type: writev2.Metadata_METRIC_TYPE_HISTOGRAM,
							},
							LabelsRefs: []uint32{
								5, 13, // __name__
								1, 2, // job
								3, 4, // instance
							},
							Exemplars: []writev2.Exemplar{
								{
									Value:     1.0,
									Timestamp: 1,
									LabelsRefs: []uint32{
										7, 11, // trace_id
										9, 12, // span_id
									},
								},
							},
						},
					},
				},
			},
			expected: func() pmetric.Metrics {
				metrics := pmetric.NewMetrics()
				rm := metrics.ResourceMetrics().AppendEmpty()
				attrs := rm.Resource().Attributes()
				attrs.PutStr("service.namespace", "production")
				attrs.PutStr("service.name", "service_a")
				attrs.PutStr("service.instance.id", "host1")

				sm := rm.ScopeMetrics().AppendEmpty()
				sm.Scope().SetName("OpenTelemetry Collector")
				sm.Scope().SetVersion("latest")

				m := sm.Metrics().AppendEmpty()
				m.SetName("request_duration_ms")
				m.SetUnit("")
				m.SetDescription("")

				hist := m.SetEmptyHistogram()
				hist.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)

				dp := hist.DataPoints().AppendEmpty()
				dp.SetCount(4)
				dp.SetSum(1)
				dp.SetTimestamp(pcommon.Timestamp(1 * int64(time.Millisecond)))

				ex := dp.Exemplars().AppendEmpty()
				ex.SetDoubleValue(1.0)
				ex.SetTimestamp(pcommon.Timestamp(1 * int64(time.Millisecond)))

				traceID, _ := hex.DecodeString("4bf92f3577b34da6a3ce929d0e0e4736")
				var tid [16]byte
				copy(tid[:], traceID)
				ex.SetTraceID(pcommon.TraceID(tid))

				spanID, _ := hex.DecodeString("00f067aa0ba902b7")
				var sid [8]byte
				copy(sid[:], spanID)
				ex.SetSpanID(pcommon.SpanID(sid))

				// Bucket counts from PositiveDeltas
				dp.BucketCounts().Append(1)
				dp.BucketCounts().Append(3)
				// Custom boundaries
				dp.ExplicitBounds().Append(1.0) // matches CustomValues

				m = sm.Metrics().AppendEmpty()
				m.SetName("latency_ms")
				m.SetUnit("")
				m.SetDescription("")

				hist = m.SetEmptyHistogram()
				hist.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)

				dp = hist.DataPoints().AppendEmpty()
				dp.SetCount(4)
				dp.SetSum(1)
				dp.SetTimestamp(pcommon.Timestamp(1 * int64(time.Millisecond)))

				ex = dp.Exemplars().AppendEmpty()
				ex.SetDoubleValue(1.0)
				ex.SetTimestamp(pcommon.Timestamp(1 * int64(time.Millisecond)))

				traceID, _ = hex.DecodeString("4bf92f3577b34da6a3ce929d0e0e4740")
				copy(tid[:], traceID)
				ex.SetTraceID(pcommon.TraceID(tid))

				spanID, _ = hex.DecodeString("fff067aa0ba902b7")
				copy(sid[:], spanID)
				ex.SetSpanID(pcommon.SpanID(sid))

				// Bucket counts from PositiveDeltas
				dp.BucketCounts().Append(1)
				dp.BucketCounts().Append(3)
				// Custom boundaries
				dp.ExplicitBounds().Append(1.0) // matches CustomValues

				return metrics
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockConsumer := new(mockConsumer)
			prwReceiver := setupMetricsReceiver(t)
			prwReceiver.nextConsumer = mockConsumer

			ts := httptest.NewServer(http.HandlerFunc(prwReceiver.handlePRW))
			defer ts.Close()

			for _, req := range tt.requests {
				pBuf := proto.NewBuffer(nil)
				err := pBuf.Marshal(req)
				require.NoError(t, err)

				resp, err := http.Post(
					ts.URL,
					fmt.Sprintf("application/x-protobuf;proto=%s", remoteapi.WriteV2MessageType),
					bytes.NewBuffer(pBuf.Bytes()),
				)
				require.NoError(t, err)
				defer resp.Body.Close()

				body, err := io.ReadAll(resp.Body)
				require.NoError(t, err)
				require.Equal(t, http.StatusNoContent, resp.StatusCode, string(body))
			}

			require.Len(t, mockConsumer.metrics, 1)
			assert.NoError(t, pmetrictest.CompareMetrics(tt.expected(), mockConsumer.metrics[0]))
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

	mockConsumer := new(mockConsumer)
	prwReceiver.nextConsumer = mockConsumer

	ts := httptest.NewServer(http.HandlerFunc(prwReceiver.handlePRW))
	defer ts.Close()

	for _, req := range []*writev2.Request{targetInfoRequest, metric1, metric2, metric1_1} {
		pBuf := proto.NewBuffer(nil)
		err := pBuf.Marshal(req)
		assert.NoError(t, err)

		resp, err := http.Post(
			ts.URL,
			fmt.Sprintf("application/x-protobuf;proto=%s", remoteapi.WriteV2MessageType),
			bytes.NewBuffer(pBuf.Bytes()),
		)
		assert.NoError(t, err)

		body, err := io.ReadAll(resp.Body)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusNoContent, resp.StatusCode, string(body))
	}

	// As target_info and metric1 have the same job/instance, they generate the same end metric: mockConsumer.metrics[0].
	assert.NoError(t, pmetrictest.CompareMetrics(expectedMetrics1, mockConsumer.metrics[0]))
	// As metric2 have different job/instance, it generates a different end metric: mockConsumer.metrics[1]. At this point, the cache is full it should evict the target_info metric to store the metric2.
	assert.NoError(t, pmetrictest.CompareMetrics(expectedMetrics2, mockConsumer.metrics[1]))
	// As just have 1 slot in the cache, but the cache for metric1 was evicted, this metric1_1 should generate a new resource metric, even having the same job/instance than the metric1.
	assert.NoError(t, pmetrictest.CompareMetrics(expectedMetrics1_1, mockConsumer.metrics[2]))
}

func buildMetaDataMapByID(ms pmetric.Metrics) map[string]map[string]any {
	result := make(map[string]map[string]any)
	for i := 0; i < ms.ResourceMetrics().Len(); i++ {
		rm := ms.ResourceMetrics().At(i)
		resourceID := identity.OfResource(rm.Resource()).String()
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			sm := rm.ScopeMetrics().At(j)
			scopeName := sm.Scope().Name()
			scopeVersion := sm.Scope().Version()
			for k := 0; k < sm.Metrics().Len(); k++ {
				m := sm.Metrics().At(k)
				metricID := fmt.Sprintf("%s:%s:%s:%s:%s",
					resourceID,
					scopeName,
					scopeVersion,
					m.Name(),
					m.Unit(),
				)
				result[metricID] = m.Metadata().AsRaw()
			}
		}
	}
	return result
}

// TestConcurrentRequestsforSameResourceAttributes asserts the receiver and its cache work even with concurrent requests
func TestConcurrentRequestsforSameResourceAttributes(t *testing.T) {
	mockConsumer := &mockConsumer{}
	prwReceiver := setupMetricsReceiver(t)
	prwReceiver.nextConsumer = mockConsumer

	ts := httptest.NewServer(http.HandlerFunc(prwReceiver.handlePRW))
	defer ts.Close()

	// Create multiple requests with the same job/instance labels (triggering cache key collision)
	createRequest := func(metricName string, value float64, timestamp int64) *writev2.Request {
		return &writev2.Request{
			Symbols: []string{
				"",                     // 0
				"__name__", metricName, // 1, 2
				"job", "test_job", // 3, 4
				"instance", "test_instance", // 5, 6
			},
			Timeseries: []writev2.TimeSeries{
				{
					Metadata:   writev2.Metadata{Type: writev2.Metadata_METRIC_TYPE_GAUGE},
					LabelsRefs: []uint32{1, 2, 3, 4, 5, 6},
					Samples:    []writev2.Sample{{Value: value, Timestamp: timestamp}},
				},
			},
		}
	}

	requests := []*writev2.Request{}
	for i := range 5 {
		requests = append(requests, createRequest("metric_"+strconv.Itoa(i+1), float64(i+1)*10, int64(i+1)*1000))
	}

	var wg sync.WaitGroup
	var httpResults []int
	var mu sync.Mutex

	for _, req := range requests {
		wg.Add(1)
		go func(request *writev2.Request) {
			defer wg.Done()

			pBuf := proto.NewBuffer(nil)
			err := pBuf.Marshal(request)
			assert.NoError(t, err)

			resp, err := http.Post(
				ts.URL,
				fmt.Sprintf("application/x-protobuf;proto=%s", remoteapi.WriteV2MessageType),
				bytes.NewBuffer(pBuf.Bytes()),
			)
			assert.NoError(t, err)
			defer resp.Body.Close()

			mu.Lock()
			httpResults = append(httpResults, resp.StatusCode)
			mu.Unlock()
		}(req)
	}
	wg.Wait()

	// Give some time for async processing
	time.Sleep(100 * time.Millisecond)

	mockConsumer.mu.Lock()
	totalDataPoints := mockConsumer.dataPoints
	mockConsumer.mu.Unlock()

	// Verify all HTTP requests succeeded
	for i, status := range httpResults {
		assert.Equal(t, http.StatusNoContent, status, "Request %d should return 204", i+1)
	}

	// The expected behavior is:
	// - All HTTP requests return 204 (success)
	// - All 5 data points are present (no data loss)
	// - We have 5 resource attributes, each with 1 data point. The resource attributes are equal since they have the same job/instance labels.
	// - The cache should have a single resource attribute.
	assert.Equal(t, 5, totalDataPoints)
	assert.Equal(t, 1, prwReceiver.rmCache.Len())

	// Verify thread safety: Check that metrics are properly consolidated without corruption
	for i, metrics := range mockConsumer.metrics {
		if metrics.DataPointCount() > 0 {
			resourceMetrics := metrics.ResourceMetrics()
			for j := 0; j < resourceMetrics.Len(); j++ {
				rm := resourceMetrics.At(j)
				scopeMetrics := rm.ScopeMetrics()
				for k := 0; k < scopeMetrics.Len(); k++ {
					scope := scopeMetrics.At(k)
					metricsCount := scope.Metrics().Len()
					if metricsCount != 1 {
						t.Errorf("Batch %d: Found %d datapoints when it should be 1", i+1, metricsCount)
					}
				}
			}
		}
	}
}

// setupMetricsReceiverWithConsumer creates a receiver with a custom consumer for testing.
func setupMetricsReceiverWithConsumer(t *testing.T, nextConsumer consumer.Metrics) *prometheusRemoteWriteReceiver {
	t.Helper()

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	prwReceiver, err := factory.CreateMetrics(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, nextConsumer)
	require.NoError(t, err)
	require.NotNil(t, prwReceiver, "metrics receiver creation failed")

	receiverID := component.MustNewID("test")
	obsrecv, err := receiverhelper.NewObsReport(receiverhelper.ObsReportSettings{
		ReceiverID:             receiverID,
		Transport:              "http",
		ReceiverCreateSettings: receivertest.NewNopSettings(metadata.Type),
	})
	require.NoError(t, err)

	prwReceiver.(*prometheusRemoteWriteReceiver).obsrecv = obsrecv
	writeReceiver := prwReceiver.(*prometheusRemoteWriteReceiver)
	t.Cleanup(func() {
		writeReceiver.rmCache.Purge()
	})

	return writeReceiver
}

func TestHandlePRWConsumerResponse(t *testing.T) {
	// Create a valid request with metrics.
	request := &writev2.Request{
		Symbols: []string{"", "__name__", "test_metric", "job", "test-job", "instance", "test-instance"},
		Timeseries: []writev2.TimeSeries{
			{
				Metadata:   writev2.Metadata{Type: writev2.Metadata_METRIC_TYPE_GAUGE},
				LabelsRefs: []uint32{1, 2, 3, 4, 5, 6},
				Samples:    []writev2.Sample{{Value: 1, Timestamp: 1}},
			},
		},
	}

	pBuf := proto.NewBuffer(nil)
	err := pBuf.Marshal(request)
	require.NoError(t, err)

	// Send raw protobuf body - in production the confighttp middleware decompresses
	// but in tests we call handlePRW directly without middleware.
	rawBody := pBuf.Bytes()

	t.Run("success returns 204", func(t *testing.T) {
		sink := &consumertest.MetricsSink{}
		prwReceiver := setupMetricsReceiverWithConsumer(t, sink)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/write", bytes.NewBuffer(rawBody))
		req.Header.Set("Content-Type", fmt.Sprintf("application/x-protobuf;proto=%s", remoteapi.WriteV2MessageType))

		w := httptest.NewRecorder()
		prwReceiver.handlePRW(w, req)
		resp := w.Result()

		assert.Equal(t, http.StatusNoContent, resp.StatusCode)
		assert.Len(t, sink.AllMetrics(), 1)
	})

	t.Run("retryable error returns 500", func(t *testing.T) {
		prwReceiver := setupMetricsReceiverWithConsumer(t, consumertest.NewErr(errors.New("temporary failure")))

		req := httptest.NewRequest(http.MethodPost, "/api/v1/write", bytes.NewBuffer(rawBody))
		req.Header.Set("Content-Type", fmt.Sprintf("application/x-protobuf;proto=%s", remoteapi.WriteV2MessageType))

		w := httptest.NewRecorder()
		prwReceiver.handlePRW(w, req)
		resp := w.Result()

		assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.Contains(t, string(body), "temporary failure")
	})

	t.Run("permanent error returns 400", func(t *testing.T) {
		prwReceiver := setupMetricsReceiverWithConsumer(t, consumertest.NewErr(consumererror.NewPermanent(errors.New("permanent failure"))))

		req := httptest.NewRequest(http.MethodPost, "/api/v1/write", bytes.NewBuffer(rawBody))
		req.Header.Set("Content-Type", fmt.Sprintf("application/x-protobuf;proto=%s", remoteapi.WriteV2MessageType))

		w := httptest.NewRecorder()
		prwReceiver.handlePRW(w, req)
		resp := w.Result()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.Contains(t, string(body), "permanent failure")
	})
}
