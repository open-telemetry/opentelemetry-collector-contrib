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

const (
	defaultBuildName    = "defaultBuildName"
	defaultBuildVersion = "defaultBuildVersion"
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
	// Save the default BuildInfo values.
	defaultBuildName := prwReceiver.settings.BuildInfo.Description
	defaultBuildVersion := prwReceiver.settings.BuildInfo.Version

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	for _, tc := range []struct {
		name                 string
		request              *writev2.Request
		expectError          string
		expectedMetrics      pmetric.Metrics
		expectedStats        remote.WriteResponseStats
		buildNameOverride    string
		buildVersionOverride string
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
			// Expected:
			// - The first two timeseries have explicit otel_scope values "scope1"/"v1" and yield gauge datapoints with {"d":"e"}.
			// - The third timeseries uses "scope2"/"v2" and yields a gauge datapoint with {"foo":"bar"}.
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
			//	This fixture has 3 timeseries:
			//  #0 job="service-x/test", instance="107cn001", value=1, ts=1 ms
			//  #1 same job/instance, value=2, ts=2 ms
			//  #2 job="foo", instance="bar", value=2, ts=2 ms
			expectedMetrics: func() pmetric.Metrics {
				expected := pmetric.NewMetrics()

				// =========================
				// Resource 1: job="service-x/test", instance="107cn001"
				// =========================
				rm1 := expected.ResourceMetrics().AppendEmpty()
				rm1.Resource().Attributes().PutStr("service.namespace", "service-x")
				rm1.Resource().Attributes().PutStr("service.name", "test")
				rm1.Resource().Attributes().PutStr("service.instance.id", "107cn001")

				sm1 := rm1.ScopeMetrics().AppendEmpty()
				sm1.Scope().SetName(buildName)
				sm1.Scope().SetVersion(buildVersion)

				// Timeseries 0: value=1, ts=1ms
				gauge0 := sm1.Metrics().AppendEmpty().SetEmptyGauge()
				dp0 := gauge0.DataPoints().AppendEmpty()
				dp0.SetTimestamp(pcommon.Timestamp(1 * int64(time.Millisecond)))
				dp0.SetDoubleValue(1.0)
				dp0.Attributes().PutStr("d", "e")
				dp0.Attributes().PutStr("foo", "bar")

				// Timeseries 1: value=2, ts=2ms
				gauge1 := sm1.Metrics().AppendEmpty().SetEmptyGauge()
				dp1 := gauge1.DataPoints().AppendEmpty()
				dp1.SetTimestamp(pcommon.Timestamp(2 * int64(time.Millisecond)))
				dp1.SetDoubleValue(2.0)
				dp1.Attributes().PutStr("d", "e")
				dp1.Attributes().PutStr("foo", "bar")

				// =========================
				// Resource 2: job="foo", instance="bar"
				// =========================
				rm2 := expected.ResourceMetrics().AppendEmpty()
				rm2.Resource().Attributes().PutStr("service.name", "foo")
				rm2.Resource().Attributes().PutStr("service.instance.id", "bar")

				sm2 := rm2.ScopeMetrics().AppendEmpty()
				sm2.Scope().SetName(buildName)
				sm2.Scope().SetVersion(buildVersion)

				// Timeseries 2: value=2, ts=2ms
				gauge2 := sm2.Metrics().AppendEmpty().SetEmptyGauge()
				dp2 := gauge2.DataPoints().AppendEmpty()
				dp2.SetTimestamp(pcommon.Timestamp(2 * int64(time.Millisecond)))
				dp2.SetDoubleValue(2.0)
				dp2.Attributes().PutStr("d", "e")
				dp2.Attributes().PutStr("foo", "bar")

				return expected
			}(),
			expectedStats: remote.WriteResponseStats{},
		},
		{
			name: "provided otel_scope_name and otel_scope_version",
			request: &writev2.Request{
				Symbols: []string{
					"", "__name__", "metric_with_scope",
					"otel_scope_name", "custom_scope",
					"otel_scope_version", "v1.0",
					"job", "service-y/custom",
					"instance", "instance-1",
				},
				Timeseries: []writev2.TimeSeries{
					{
						Metadata:   writev2.Metadata{Type: writev2.Metadata_METRIC_TYPE_GAUGE},
						LabelsRefs: []uint32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
						Samples:    []writev2.Sample{{Value: 10, Timestamp: 100}},
					},
				},
			},
			expectedMetrics: func() pmetric.Metrics {
				expected := pmetric.NewMetrics()
				rm := expected.ResourceMetrics().AppendEmpty()
				parseJobAndInstance(rm.Resource().Attributes(), "service-y/custom", "instance-1")
				sm := rm.ScopeMetrics().AppendEmpty()
				sm.Scope().SetName("custom_scope")
				sm.Scope().SetVersion("v1.0")
				_ = sm.Metrics().AppendEmpty().SetEmptyGauge().DataPoints().AppendEmpty()
				return expected
			}(),
			expectedStats: remote.WriteResponseStats{},
		},
		{
			name: "missing otel_scope_name/version falls back to BuildInfo",
			// When missing, ls.Get returns "" so the defaults from BuildInfo are preserved.
			request: &writev2.Request{
				Symbols: []string{
					"",                // index 0
					"__name__",        // index 1
					"metric_no_scope", // index 2
					"job",             // index 3
					"service-z/xyz",   // index 4
					"instance",        // index 5
					"inst-42",         // index 6
					"d",               // index 7
					"e",               // index 8
					"foo",             // index 9
					"bar",             // index 10
				},
				Timeseries: []writev2.TimeSeries{
					{
						Metadata:   writev2.Metadata{Type: writev2.Metadata_METRIC_TYPE_GAUGE},
						LabelsRefs: []uint32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
						Samples:    []writev2.Sample{{Value: 5, Timestamp: 50}},
					},
				},
			},
			expectedMetrics: func() pmetric.Metrics {
				expected := pmetric.NewMetrics()
				rm := expected.ResourceMetrics().AppendEmpty()
				parseJobAndInstance(rm.Resource().Attributes(), "service-z/xyz", "inst-42")
				sm := rm.ScopeMetrics().AppendEmpty()
				// Expect fallback to default BuildInfo.
				sm.Scope().SetName(defaultBuildName)
				sm.Scope().SetVersion(defaultBuildVersion)
				m := sm.Metrics().AppendEmpty().SetEmptyGauge()
				dp := m.DataPoints().AppendEmpty()
				dp.Attributes().PutStr("d", "e")
				dp.Attributes().PutStr("foo", "bar")
				return expected
			}(),
			expectedStats: remote.WriteResponseStats{},
		},
		{
			name: "custom BuildInfo used when no scope provided",
			// Even if BuildInfo is overridden, if no otel_scope is provided, the overridden defaults are used.
			buildNameOverride:    "customBuildName",
			buildVersionOverride: "customBuildVersion",
			request: &writev2.Request{
				Symbols: []string{
					"", "__name__", "metric_custom",
					"job", "service-custom/svc",
					"instance", "inst-custom",
					"a", "b",
				},
				Timeseries: []writev2.TimeSeries{
					{
						Metadata:   writev2.Metadata{Type: writev2.Metadata_METRIC_TYPE_GAUGE},
						LabelsRefs: []uint32{1, 2, 3, 4, 5, 6, 7, 8},
						Samples:    []writev2.Sample{{Value: 42, Timestamp: 100}},
					},
				},
			},
			expectedMetrics: func() pmetric.Metrics {
				expected := pmetric.NewMetrics()
				rm := expected.ResourceMetrics().AppendEmpty()
				parseJobAndInstance(rm.Resource().Attributes(), "service-custom/svc", "inst-custom")
				sm := rm.ScopeMetrics().AppendEmpty()
				// Expect the overridden BuildInfo values.
				sm.Scope().SetName("customBuildName")
				sm.Scope().SetVersion("customBuildVersion")
				m := sm.Metrics().AppendEmpty().SetEmptyGauge()
				dp := m.DataPoints().AppendEmpty()
				dp.Attributes().PutStr("a", "b")
				return expected
			}(),
			expectedStats: remote.WriteResponseStats{},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.buildNameOverride != "" || tc.buildVersionOverride != "" {
				prwReceiver.settings.BuildInfo.Description = tc.buildNameOverride
				prwReceiver.settings.BuildInfo.Version = tc.buildVersionOverride
			}
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
