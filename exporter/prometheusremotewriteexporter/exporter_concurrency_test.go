// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewriteexporter

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/prompb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusremotewriteexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/testdata"
)

// Test everything works when there is more than one goroutine calling PushMetrics.
// Today we only use 1 worker per exporter, but the intention of this test is to future-proof in case it changes.
func Test_PushMetricsConcurrent(t *testing.T) {
	n := 1000
	ms := make([]pmetric.Metrics, n)
	testIDKey := "test_id"
	for i := 0; i < n; i++ {
		m := testdata.GenerateMetricsOneMetric()
		dps := m.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Sum().DataPoints()
		for j := 0; j < dps.Len(); j++ {
			dp := dps.At(j)
			dp.Attributes().PutInt(testIDKey, int64(i))
		}
		ms[i] = m
	}
	received := make(map[int]prompb.TimeSeries)
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		assert.NotNil(t, body)
		if len(body) == 0 {
			// No content, nothing to do. The request is just checking that the server is up.
			return
		}
		// Receives the http requests and unzip, unmarshalls, and extracts TimeSeries
		assert.Equal(t, "0.1.0", r.Header.Get("X-Prometheus-Remote-Write-Version"))
		assert.Equal(t, "snappy", r.Header.Get("Content-Encoding"))
		var unzipped []byte

		dest, err := snappy.Decode(unzipped, body)
		assert.NoError(t, err)

		wr := &prompb.WriteRequest{}
		ok := proto.Unmarshal(dest, wr)
		assert.NoError(t, ok)
		assert.Len(t, wr.Timeseries, 2)
		ts := wr.Timeseries[0]
		foundLabel := false
		for _, label := range ts.Labels {
			if label.Name == testIDKey {
				id, err := strconv.Atoi(label.Value)
				assert.NoError(t, err)
				mu.Lock()
				_, ok := received[id]
				assert.False(t, ok) // fail if we already saw it
				received[id] = ts
				mu.Unlock()
				foundLabel = true
				break
			}
		}
		assert.True(t, foundLabel)
		w.WriteHeader(http.StatusOK)
	}))

	defer server.Close()

	// Adjusted retry settings for faster testing
	retrySettings := configretry.BackOffConfig{
		Enabled:         true,
		InitialInterval: 100 * time.Millisecond, // Shorter initial interval
		MaxInterval:     1 * time.Second,        // Shorter max interval
		MaxElapsedTime:  2 * time.Second,        // Shorter max elapsed time
	}
	clientConfig := confighttp.NewDefaultClientConfig()
	clientConfig.Endpoint = server.URL
	clientConfig.ReadBufferSize = 0
	clientConfig.WriteBufferSize = 512 * 1024
	cfg := &Config{
		Namespace:         "",
		ClientConfig:      clientConfig,
		MaxBatchSizeBytes: 3000000,
		RemoteWriteQueue:  RemoteWriteQueue{NumConsumers: 1},
		TargetInfo: &TargetInfo{
			Enabled: true,
		},
		BackOffConfig:       retrySettings,
		RemoteWriteProtoMsg: config.RemoteWriteProtoMsgV1,
	}

	assert.NotNil(t, cfg)
	set := exportertest.NewNopSettings(metadata.Type)
	prwe, nErr := newPRWExporter(cfg, set)

	require.NoError(t, nErr)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, prwe.Start(ctx, componenttest.NewNopHost()))
	defer func() {
		require.NoError(t, prwe.Shutdown(ctx))
	}()

	// Ensure that the test server is up before making the requests
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		resp, checkRequestErr := http.Get(server.URL)
		require.NoError(c, checkRequestErr)
		assert.NoError(c, resp.Body.Close())
	}, 15*time.Second, 100*time.Millisecond)

	var wg sync.WaitGroup
	wg.Add(n)
	maxConcurrentGoroutines := runtime.NumCPU() * 4
	semaphore := make(chan struct{}, maxConcurrentGoroutines)
	for _, m := range ms {
		semaphore <- struct{}{}
		go func() {
			defer func() {
				<-semaphore
				wg.Done()
			}()

			err := prwe.PushMetrics(ctx, m)
			assert.NoError(t, err)
		}()
	}
	wg.Wait()
	assert.Len(t, received, n)
}
