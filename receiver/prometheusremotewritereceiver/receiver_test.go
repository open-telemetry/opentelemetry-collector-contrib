// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewritereceiver

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/prometheus/prometheus/prompb"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

var url = "http://localhost:9090"

func MakeRequest(ctx context.Context, url string, writeRequest *prompb.WriteRequest) (int, error) {
	var code int
	data, err := proto.Marshal(writeRequest)
	if err != nil {
		code = 500
		return code, fmt.Errorf("marshall error: %w", err)
	}

	encoded := snappy.Encode(nil, data)

	body := bytes.NewReader(encoded)
	req, err := http.NewRequest(http.MethodPost, url, body)
	if err != nil {
		return code, err
	}
	req.Header.Set("Content-Type", "application/x-protobuf")
	req.Header.Set("Content-Encoding", "snappy")

	httpClient := http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := httpClient.Do(req.WithContext(ctx))
	if err != nil {
		return 500, err
	}

	code = resp.StatusCode

	defer resp.Body.Close()

	return code, err
}

var (
	now       = time.Now()
	nowMillis = now.UnixNano() / int64(time.Millisecond)
)

var series = []prompb.TimeSeries{
	{
		Labels: []prompb.Label{
			{Name: "__name__", Value: "http_server_requests_seconds_sum"},
			{Name: "method", Value: "GET"},
		},
		Samples: []prompb.Sample{{Value: 1.0, Timestamp: nowMillis}},
	},
	{
		Labels: []prompb.Label{
			{Name: "__name__", Value: "http_server_requests_seconds_count"},
			{Name: "method", Value: "GET"},
		},
		Samples: []prompb.Sample{{Value: 2.0, Timestamp: nowMillis}},
	},
	{
		Labels: []prompb.Label{
			{Name: "__name__", Value: "pdc_jvm_memory_used_bytes"},
			{Name: "xxx", Value: "yyy"},
		},
		Samples: []prompb.Sample{{Value: 0.0, Timestamp: nowMillis}},
	},
	{
		Labels: []prompb.Label{
			{Name: "__name__", Value: "metric_point_from_the_past"},
			{Name: "xxx", Value: "yyy"},
		},
		Samples: []prompb.Sample{{Value: 0.0, Timestamp: now.Add(-time.Hour*24).UnixNano() / int64(time.Millisecond)}},
	},
}

func TestPrometheusRemoteWriteReceiver(t *testing.T) {
	ctx := context.Background()
	cms := new(consumertest.MetricsSink)

	config := createDefaultConfig().(*Config)

	receiver, _ := NewReceiver(
		receivertest.NewNopSettings(),
		config,
		cms)

	require.NoError(t, receiver.Start(ctx, componenttest.NewNopHost()))

	request := &prompb.WriteRequest{
		Timeseries: series,
	}

	r, err := MakeRequest(context.Background(), url, request)

	require.Nil(t, err)
	require.Equal(t, http.StatusAccepted, r)
	require.Equal(t, 4, cms.AllMetrics()[0].ResourceMetrics().Len())

	require.Equal(t, pmetric.MetricTypeSum, cms.AllMetrics()[0].ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Type())
	require.Equal(t, true, cms.AllMetrics()[0].ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().IsMonotonic())
	require.Equal(t, pmetric.AggregationTemporalityCumulative, cms.AllMetrics()[0].ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().AggregationTemporality())

	require.Equal(t, pmetric.MetricTypeSum, cms.AllMetrics()[0].ResourceMetrics().At(1).ScopeMetrics().At(0).Metrics().At(0).Type())
	require.Equal(t, true, cms.AllMetrics()[0].ResourceMetrics().At(1).ScopeMetrics().At(0).Metrics().At(0).Sum().IsMonotonic())
	require.Equal(t, pmetric.AggregationTemporalityCumulative, cms.AllMetrics()[0].ResourceMetrics().At(1).ScopeMetrics().At(0).Metrics().At(0).Sum().AggregationTemporality())

	require.Equal(t, pmetric.MetricTypeGauge, cms.AllMetrics()[0].ResourceMetrics().At(2).ScopeMetrics().At(0).Metrics().At(0).Type())

	require.Equal(t, pmetric.MetricTypeEmpty, cms.AllMetrics()[0].ResourceMetrics().At(3).ScopeMetrics().At(0).Metrics().At(0).Type())
}
