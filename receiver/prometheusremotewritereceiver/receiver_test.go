// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewritereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusremotewritereceiver"

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	promconfig "github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/model/histogram"
	writev2 "github.com/prometheus/prometheus/prompb/io/prometheus/write/v2"
	"github.com/prometheus/prometheus/storage/remote"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

var (
	testHistogram = histogram.Histogram{
		Schema:          2,
		ZeroThreshold:   1e-128,
		ZeroCount:       0,
		Count:           0,
		Sum:             20,
		PositiveSpans:   []histogram.Span{{Offset: 0, Length: 1}},
		PositiveBuckets: []int64{1},
		NegativeSpans:   []histogram.Span{{Offset: 0, Length: 1}},
		NegativeBuckets: []int64{-1},
	}

	writeV2RequestFixture = &writev2.Request{
		Symbols: []string{"", "__name__", "test_metric1", "b", "c", "baz", "qux", "d", "e", "foo", "bar", "f", "g", "h", "i", "Test gauge for test purposes", "Maybe op/sec who knows (:", "Test counter for test purposes"},
		Timeseries: []writev2.TimeSeries{
			{
				LabelsRefs: []uint32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, // Symbolized writeRequestFixture.Timeseries[0].Labels
				Metadata: writev2.Metadata{
					Type: writev2.Metadata_METRIC_TYPE_GAUGE, // writeV2RequestSeries1Metadata.Type.

					HelpRef: 15, // Symbolized writeV2RequestSeries1Metadata.Help.
					UnitRef: 16, // Symbolized writeV2RequestSeries1Metadata.Unit.
				},
				Samples:    []writev2.Sample{{Value: 1, Timestamp: 1}},
				Exemplars:  []writev2.Exemplar{{LabelsRefs: []uint32{11, 12}, Value: 1, Timestamp: 1}},
				Histograms: []writev2.Histogram{writev2.FromIntHistogram(1, &testHistogram), writev2.FromFloatHistogram(2, testHistogram.ToFloat(nil))},
			},
			{
				LabelsRefs: []uint32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, // Same series as first.
				Metadata: writev2.Metadata{
					Type: writev2.Metadata_METRIC_TYPE_COUNTER, // writeV2RequestSeries2Metadata.Type.

					HelpRef: 17, // Symbolized writeV2RequestSeries2Metadata.Help.
					// No unit.
				},
				Samples:    []writev2.Sample{{Value: 2, Timestamp: 2}},
				Exemplars:  []writev2.Exemplar{{LabelsRefs: []uint32{13, 14}, Value: 2, Timestamp: 2}},
				Histograms: []writev2.Histogram{writev2.FromIntHistogram(3, &testHistogram), writev2.FromFloatHistogram(4, testHistogram.ToFloat(nil))},
			},
		},
	}
)

func setupMetricsReceiver(t *testing.T) *prometheusRemoteWriteReceiver {
	t.Helper()

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	prwReceiver, err := factory.CreateMetrics(context.Background(), receivertest.NewNopSettings(), cfg, consumertest.NewNop())
	assert.NoError(t, err)
	assert.NotNil(t, prwReceiver, "metrics receiver creation failed")

	return prwReceiver.(*prometheusRemoteWriteReceiver)
}

func setupServer(t *testing.T) {
	t.Helper()

	prwReceiver := setupMetricsReceiver(t)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	assert.NoError(t, prwReceiver.Start(ctx, componenttest.NewNopHost()))
	t.Cleanup(func() {
		assert.NoError(t, prwReceiver.Shutdown(ctx), "Must not error shutting down")
	})
}

func TestHandlePRWContentTypeNegotiation(t *testing.T) {
	setupServer(t)

	for _, tc := range []struct {
		name         string
		contentType  string
		extectedCode int
	}{
		{
			name:         "no content type",
			contentType:  "",
			extectedCode: http.StatusUnsupportedMediaType,
		},
		{
			name:         "unsupported content type",
			contentType:  "application/json",
			extectedCode: http.StatusUnsupportedMediaType,
		},
		{
			name:         "x-protobuf/no proto parameter",
			contentType:  "application/x-protobuf",
			extectedCode: http.StatusUnsupportedMediaType,
		},
		{
			name:         "x-protobuf/v1 proto parameter",
			contentType:  fmt.Sprintf("application/x-protobuf;proto=%s", promconfig.RemoteWriteProtoMsgV1),
			extectedCode: http.StatusUnsupportedMediaType,
		},
		{
			name:         "x-protobuf/v2 proto parameter",
			contentType:  fmt.Sprintf("application/x-protobuf;proto=%s", promconfig.RemoteWriteProtoMsgV2),
			extectedCode: http.StatusNoContent,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := writev2.Request{}
			pBuf := proto.NewBuffer(nil)
			err := pBuf.Marshal(&body)
			assert.NoError(t, err)

			var compressedBody []byte
			snappy.Encode(compressedBody, pBuf.Bytes())
			req, err := http.NewRequest(http.MethodPost, "http://localhost:9090/api/v1/write", bytes.NewBuffer(compressedBody))
			assert.NoError(t, err)

			req.Header.Set("Content-Type", tc.contentType)
			req.Header.Set("Content-Encoding", "snappy")
			resp, err := http.DefaultClient.Do(req)
			assert.NoError(t, err)

			assert.Equal(t, tc.extectedCode, resp.StatusCode)
			if tc.extectedCode == http.StatusNoContent { // We went until the end
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
			name:    "valid request",
			request: writeV2RequestFixture,
			expectedMetrics: func() pmetric.Metrics {
				expected := pmetric.NewMetrics()
				rmAttributes := expected.ResourceMetrics().AppendEmpty().Resource().Attributes()
				rmAttributes.PutStr("b", "c")
				rmAttributes.PutStr("baz", "qux")
				rmAttributes.PutStr("d", "e")
				rmAttributes.PutStr("foo", "bar")
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
