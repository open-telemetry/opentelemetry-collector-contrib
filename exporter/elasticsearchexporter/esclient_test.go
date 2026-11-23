// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elasticsearchexporter

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/elastic/elastic-transport-go/v8/elastictransport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/testdata"
	"go.uber.org/zap"
)

func TestTimeoutInterceptor(t *testing.T) {
	for _, tc := range []struct {
		name        string
		timeout     time.Duration
		bodyLatency time.Duration
	}{
		{
			name:    "zero",
			timeout: 30 * time.Second,
		},
		{
			name:        "fast",
			timeout:     30 * time.Second,
			bodyLatency: time.Millisecond,
		},
		{
			name:        "slow",
			timeout:     30 * time.Second,
			bodyLatency: time.Second,
		},
		{
			name:    "zero_timeout",
			timeout: 0,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := newESTestServerBulkHandlerFunc(t, currentESVersion, func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
				if f, ok := w.(http.Flusher); ok {
					f.Flush()
				}
				if tc.bodyLatency > 0 {
					time.Sleep(tc.bodyLatency)
				}
				_, _ = w.Write([]byte(`{"took":1,"errors":false,"items":[{"create":{"_index":"logs-generic-default","status":201}}]}`))
			})
			exporter := newTestLogsExporter(t, server.URL, func(cfg *Config) {
				cfg.ClientConfig.Timeout = tc.timeout
				cfg.QueueBatchConfig.Get().WaitForResult = true
			})
			mustSendLogs(t, exporter, testdata.GenerateLogs(10))
		})
	}
}

func TestComponentStatus(t *testing.T) {
	statusChan := make(chan *componentstatus.Event, 1)
	reporter := &testStatusReporter{statusChan}
	esLogger := clientLogger{
		Logger:          zap.New(nil),
		logRequestBody:  false,
		logResponseBody: false,
		componentHost:   reporter,
	}

	// Pass in an error and make sure it's sent to the component status reporter
	_ = esLogger.LogRoundTrip(nil, nil, io.EOF, time.Now(), 0)
	select {
	case event := <-statusChan:
		assert.ErrorIs(t, event.Err(), io.EOF, "LogRoundTrip should report a component status error wrapping its error parameter")
		assert.Equal(t, componentstatus.StatusRecoverableError, event.Status(), "LogRoundTrip on an error parameter should report a recoverable error")
	default:
		require.Fail(t, "LogRoundTrip with an error should report a recoverable error status")
	}

	// Pass in an http error status and make sure it's sent to the component status reporter
	_ = esLogger.LogRoundTrip(
		&http.Request{URL: &url.URL{}},
		&http.Response{StatusCode: http.StatusUnauthorized, Status: "401 Unauthorized"},
		nil, time.Now(), 0,
	)
	select {
	case event := <-statusChan:
		err := event.Err()
		require.Error(t, err, "LogRoundTrip with an http error status should report a component status error")
		assert.Contains(t, err.Error(), "401 Unauthorized", "LogRoundTrip with an http error status should include the status in its error state")
		assert.Equal(t, componentstatus.StatusRecoverableError, event.Status(), "LogRoundTrip with an http error status should report a recoverable error")
	default:
		require.Fail(t, "LogRoundTrip with an http error code should report a recoverable error status")
	}

	// Pass in a 409 (duplicate document) and make sure it doesn't report a new status
	_ = esLogger.LogRoundTrip(
		&http.Request{URL: &url.URL{}},
		&http.Response{StatusCode: http.StatusConflict, Status: "409 duplicate"},
		nil, time.Now(), 0,
	)
	select {
	case <-statusChan:
		assert.Fail(t, "LogRoundTrip with a 409 should not change the component status")
	default:
	}

	// Pass in an http success status and make sure the component status returns to OK
	_ = esLogger.LogRoundTrip(
		&http.Request{URL: &url.URL{}},
		&http.Response{StatusCode: http.StatusOK}, nil, time.Now(), 0,
	)
	select {
	case event := <-statusChan:
		assert.NoError(t, event.Err(), "LogRoundTrip with a success status shouldn't report a component status error")
		assert.Equal(t, componentstatus.StatusOK, event.Status(), "LogRoundTrip with a success status should report component status OK")
	default:
		require.Fail(t, "LogRoundTrip with an http success should report component status OK")
	}
}

type testStatusReporter struct {
	statusChan chan *componentstatus.Event
}

func (tsr *testStatusReporter) Report(event *componentstatus.Event) {
	tsr.statusChan <- event
}

func (*testStatusReporter) GetExtensions() map[component.ID]component.Component {
	return make(map[component.ID]component.Component)
}

func TestLoadBalanceDisabled(t *testing.T) {
	t.Run("sends to first endpoint", func(t *testing.T) {
		first, firstRequests := newCountingServer(t)
		second, secondRequests := newCountingServer(t)
		client := newLoadBalanceDisabledTestClient(t, []string{first.URL, second.URL})

		for range 4 {
			_, err := performBulkRequest(t, client)
			require.NoError(t, err)
		}
		assert.Equal(t, int32(4), firstRequests.Load())
		assert.Equal(t, int32(0), secondRequests.Load())
	})

	t.Run("fails over and stays on next endpoint", func(t *testing.T) {
		down := unreachableEndpoint(t)
		up, upRequests := newCountingServer(t)
		client := newLoadBalanceDisabledTestClient(t, []string{down, up.URL})

		for range 3 {
			res, err := performBulkRequest(t, client)
			require.NoError(t, err)
			assert.Equal(t, http.StatusOK, res.StatusCode)
		}
		assert.Equal(t, int32(3), upRequests.Load())
	})
}

func newCountingServer(t *testing.T) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)
	return server, &requests
}

func unreachableEndpoint(t *testing.T) string {
	t.Helper()
	server := httptest.NewServer(http.NotFoundHandler())
	server.Close()
	return server.URL
}

func newLoadBalanceDisabledTestClient(t *testing.T, endpoints []string) elastictransport.Interface {
	t.Helper()
	cfg := createDefaultConfig().(*Config)
	cfg.Endpoints = endpoints
	cfg.LoadBalance = false
	cfg.Retry.InitialInterval = time.Millisecond
	cfg.Retry.MaxInterval = time.Millisecond
	client, err := newElasticsearchClient(t.Context(), cfg, componenttest.NewNopHost(), componenttest.NewNopTelemetrySettings(), "")
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, client.(elastictransport.Closeable).Close(context.WithoutCancel(t.Context())))
	})
	return client
}

func performBulkRequest(t *testing.T, client elastictransport.Interface) (*http.Response, error) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, "http://localhost/_bulk", http.NoBody)
	require.NoError(t, err)
	res, err := client.Perform(req)
	if res != nil {
		t.Cleanup(func() { res.Body.Close() })
	}
	return res, err
}
