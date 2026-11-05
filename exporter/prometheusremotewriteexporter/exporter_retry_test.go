// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewriteexporter

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	remoteapi "github.com/prometheus/client_golang/exp/api/remote"
	"github.com/prometheus/prometheus/prompb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exportertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusremotewriteexporter/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/testutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/testdata"
)

// hang blocks until the client gives up. The body is read first, as the server only notices that
// the client went away once the request body was consumed.
func hang(r *http.Request) {
	_, _ = io.Copy(io.Discard, r.Body)
	select {
	case <-r.Context().Done():
	case <-time.After(10 * time.Second):
	}
}

// serviceUnavailable is the handler of a backend that is down.
func serviceUnavailable(_ int64, w http.ResponseWriter, _ *http.Request) {
	http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
}

// flakyServer is a remote write endpoint whose responses are decided by the handler.
type flakyServer struct {
	*httptest.Server

	attempts atomic.Int64
}

func newFlakyServer(t *testing.T, handler func(attempt int64, w http.ResponseWriter, r *http.Request)) *flakyServer {
	t.Helper()
	s := &flakyServer{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt := s.attempts.Add(1)
		handler(attempt, w, r)
	}))
	t.Cleanup(s.Close)
	return s
}

// retryTestConfig returns a configuration with short retry intervals, so that the tests run quickly.
func retryTestConfig(endpoint string, timeout, maxElapsedTime time.Duration) *Config {
	cfg := createDefaultConfig().(*Config)
	cfg.HTTP.Endpoint = endpoint
	// Keep the HTTP client timeout out of the way, the exporter timeout is what the tests exercise.
	cfg.HTTP.Timeout = time.Minute
	cfg.TimeoutSettings.Timeout = timeout
	cfg.BackOffConfig = configretry.BackOffConfig{
		Enabled:             true,
		InitialInterval:     10 * time.Millisecond,
		RandomizationFactor: 0,
		Multiplier:          1.5,
		MaxInterval:         50 * time.Millisecond,
		MaxElapsedTime:      maxElapsedTime,
	}
	return cfg
}

// newStartedExporter creates the exporter with the factory, like the collector does, so that the
// exporterhelper options (timeout, queue) are part of the test.
func newStartedExporter(t *testing.T, cfg *Config) exporter.Metrics {
	t.Helper()
	exp, err := NewFactory().CreateMetrics(t.Context(), exportertest.NewNopSettings(metadata.Type), cfg)
	require.NoError(t, err)
	require.NoError(t, exp.Start(t.Context(), componenttest.NewNopHost()))
	t.Cleanup(func() {
		done := make(chan error, 1)
		go func() { done <- exp.Shutdown(context.Background()) }() //nolint:usetesting // the test context is already canceled
		select {
		case err := <-done:
			assert.NoError(t, err)
		case <-time.After(10 * time.Second):
			t.Error("the shutdown of the exporter did not complete")
		}
	})
	return exp
}

// newTestPRWExporter creates the exporter without the exporterhelper, to call execute directly.
func newTestPRWExporter(t *testing.T, cfg *Config) *prwExporter {
	t.Helper()
	prwe, err := newPRWExporter(cfg, exportertest.NewNopSettings(metadata.Type))
	require.NoError(t, err)
	require.NoError(t, prwe.Start(t.Context(), componenttest.NewNopHost()))
	t.Cleanup(func() { assert.NoError(t, prwe.Shutdown(context.Background())) }) //nolint:usetesting // the test context is already canceled
	return prwe
}

func emptyRequestBuffer(t *testing.T) []byte {
	t.Helper()
	buf, err := (&buffer{}).MarshalAndEncode(&prompb.WriteRequest{})
	require.NoError(t, err)
	return buf
}

// TestRetriesOutliveTimeout verifies that the timeout bounds a single attempt and not the whole
// retry loop: data must be delivered when the backend recovers after longer than the timeout.
func TestRetriesOutliveTimeout(t *testing.T) {
	for _, tt := range []struct {
		name    string
		protoV2 bool
	}{
		{name: "remote write 1"},
		{name: "remote write 2", protoV2: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if tt.protoV2 {
				defer testutil.SetFeatureGateForTest(t, metadata.ExporterPrometheusremotewritexporterEnableSendingRW2FeatureGate, true)()
			}

			const timeout = 100 * time.Millisecond
			outage := 6 * timeout

			// The backend answers 503 until it was unavailable for outage, counted from the first
			// request, which is longer than the timeout.
			var (
				firstRequest sync.Once
				restoredAt   time.Time
				delivered    atomic.Int64
				contentType  atomic.Value
			)
			server := newFlakyServer(t, func(_ int64, w http.ResponseWriter, r *http.Request) {
				firstRequest.Do(func() { restoredAt = time.Now().Add(outage) })
				if time.Now().Before(restoredAt) {
					http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
					return
				}
				contentType.Store(r.Header.Get("Content-Type"))
				delivered.Add(1)
				w.WriteHeader(http.StatusOK)
			})

			cfg := retryTestConfig(server.URL, timeout, 30*time.Second)
			if tt.protoV2 {
				cfg.RemoteWriteProtoMsg = remoteapi.WriteV2MessageType
			}
			exp := newStartedExporter(t, cfg)

			require.NoError(t, exp.ConsumeMetrics(t.Context(), testdata.GenerateMetricsOneMetric()))

			// Before the fix the request was dropped as soon as the timeout elapsed, so it was
			// never delivered once the backend came back.
			require.Eventually(t, func() bool { return delivered.Load() > 0 }, 10*time.Second, 10*time.Millisecond,
				"the data must be delivered after the backend recovered")
			assert.Greater(t, server.attempts.Load(), int64(2), "the request must be retried")
			if tt.protoV2 {
				assert.Equal(t, "application/x-protobuf;proto=io.prometheus.write.v2.Request", contentType.Load())
			} else {
				assert.Equal(t, "application/x-protobuf", contentType.Load())
			}
		})
	}
}

// TestAttemptTimeout verifies that the timeout is applied to every attempt: attempts to a hanging
// backend are cut after the timeout and retried, instead of waiting for the HTTP client timeout.
func TestAttemptTimeout(t *testing.T) {
	const (
		timeout    = 150 * time.Millisecond
		hungServed = 2
	)
	var (
		mu       sync.Mutex
		hungFor  []time.Duration
		delivery atomic.Int64
	)
	server := newFlakyServer(t, func(attempt int64, w http.ResponseWriter, r *http.Request) {
		if attempt <= hungServed {
			start := time.Now()
			hang(r)
			mu.Lock()
			hungFor = append(hungFor, time.Since(start))
			mu.Unlock()
			return
		}
		delivery.Add(1)
		w.WriteHeader(http.StatusOK)
	})

	prwe := newTestPRWExporter(t, retryTestConfig(server.URL, timeout, 30*time.Second))

	start := time.Now()
	require.NoError(t, prwe.execute(t.Context(), emptyRequestBuffer(t)))
	assert.Less(t, time.Since(start), 5*time.Second, "attempts must be cut by the timeout, not by the HTTP client timeout")

	assert.Equal(t, int64(1), delivery.Load())
	assert.Equal(t, int64(hungServed+1), server.attempts.Load())
	// The handlers record their duration when they return, which can be after the export returned.
	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(hungFor) == hungServed
	}, 5*time.Second, 5*time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	for _, d := range hungFor {
		assert.GreaterOrEqual(t, d, timeout/4, "the attempt was cut too early")
		assert.Less(t, d, 5*timeout, "the attempt was not cut by the timeout")
	}
}

// TestAttemptTimeoutRetryDisabled verifies that, without retries, the timeout bounds the single
// attempt and that the error is not permanent.
func TestAttemptTimeoutRetryDisabled(t *testing.T) {
	const timeout = 100 * time.Millisecond
	server := newFlakyServer(t, func(_ int64, _ http.ResponseWriter, r *http.Request) { hang(r) })

	cfg := retryTestConfig(server.URL, timeout, 30*time.Second)
	cfg.BackOffConfig.Enabled = false
	prwe := newTestPRWExporter(t, cfg)

	start := time.Now()
	err := prwe.execute(t.Context(), emptyRequestBuffer(t))
	require.Error(t, err)
	assert.Less(t, time.Since(start), 5*time.Second)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assertNonPermanentError(t, err)
	assert.Equal(t, int64(1), server.attempts.Load(), "must not retry when retry_on_failure is disabled")
}

// TestMaxElapsedTimeBoundsRetries verifies that the retries stop after max_elapsed_time, which is
// longer than the timeout, and that the request fails with a non-permanent error. The queue is
// disabled so that the error of the export is returned to the caller.
func TestMaxElapsedTimeBoundsRetries(t *testing.T) {
	const (
		timeout        = 20 * time.Millisecond
		maxElapsedTime = 500 * time.Millisecond
	)
	server := newFlakyServer(t, func(_ int64, w http.ResponseWriter, _ *http.Request) {
		http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
	})
	cfg := retryTestConfig(server.URL, timeout, maxElapsedTime)
	cfg.RemoteWriteQueue.Enabled = false
	exp := newStartedExporter(t, cfg)

	start := time.Now()
	err := exp.ConsumeMetrics(t.Context(), testdata.GenerateMetricsOneMetric())
	elapsed := time.Since(start)

	require.Error(t, err)
	assertNonPermanentError(t, err)
	// Before the fix the exporterhelper canceled the export after the timeout.
	assert.Greater(t, elapsed, 5*timeout, "retries must continue longer than the timeout")
	assert.Less(t, elapsed, maxElapsedTime+time.Second, "retries must stop after max_elapsed_time")
	assert.Greater(t, server.attempts.Load(), int64(3))
}

// TestRetryBackoffGrows verifies that the delay between retries grows up to max_interval. The
// delay used to stay at initial_interval because max_interval was not passed to the backoff.
func TestRetryBackoffGrows(t *testing.T) {
	server := newFlakyServer(t, func(_ int64, w http.ResponseWriter, _ *http.Request) {
		http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
	})
	cfg := retryTestConfig(server.URL, 0, time.Second)
	cfg.BackOffConfig.InitialInterval = 20 * time.Millisecond
	cfg.BackOffConfig.Multiplier = 2
	cfg.BackOffConfig.MaxInterval = time.Second
	prwe := newTestPRWExporter(t, cfg)

	require.Error(t, prwe.execute(t.Context(), emptyRequestBuffer(t)))

	// Waits of 20, 40, 80, 160, 320 and 640ms: 6 attempts fit in 1s. With a constant 20ms delay
	// there would be about 50.
	attempts := server.attempts.Load()
	assert.GreaterOrEqual(t, attempts, int64(4))
	assert.LessOrEqual(t, attempts, int64(10))
}

// TestRetryStopped verifies that, after retries were stopped, a request is attempted once.
func TestRetryStopped(t *testing.T) {
	server := newFlakyServer(t, func(_ int64, w http.ResponseWriter, _ *http.Request) {
		http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
	})
	prwe := newTestPRWExporter(t, retryTestConfig(server.URL, time.Second, time.Hour))

	prwe.startShutdown(t.Context())
	prwe.startShutdown(t.Context()) // must be safe to call twice

	err := prwe.execute(t.Context(), emptyRequestBuffer(t))
	require.Error(t, err)
	assertNonPermanentError(t, err)
	assert.Equal(t, int64(1), server.attempts.Load())
}

// TestShutdownInterruptsRetries verifies that a shutdown does not wait for the retries of an
// in-flight request, and that the error of the last attempt is returned.
func TestShutdownInterruptsRetries(t *testing.T) {
	server := newFlakyServer(t, func(_ int64, w http.ResponseWriter, _ *http.Request) {
		http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
	})
	prwe := newTestPRWExporter(t, retryTestConfig(server.URL, time.Second, time.Hour))

	buf := emptyRequestBuffer(t)
	errCh := make(chan error, 1)
	go func() { errCh <- prwe.execute(context.Background(), buf) }() //nolint:usetesting // must outlive the test context

	require.Eventually(t, func() bool { return server.attempts.Load() >= 2 }, 5*time.Second, 5*time.Millisecond)

	start := time.Now()
	require.NoError(t, prwe.Shutdown(context.Background())) //nolint:usetesting // the test context is already canceled
	select {
	case err := <-errCh:
		require.Error(t, err)
		assertNonPermanentError(t, err)
		assert.NotErrorIs(t, err, context.Canceled, "the error of the last attempt must be returned")
	case <-time.After(5 * time.Second):
		t.Fatal("the retries were not interrupted by the shutdown")
	}
	assert.Less(t, time.Since(start), 5*time.Second)
}

// TestExporterShutdownWhileBackendUnavailable verifies that the exporter created by the factory
// shuts down promptly, including the flush of the queue, while the backend is unavailable and
// requests are being retried.
func TestExporterShutdownWhileBackendUnavailable(t *testing.T) {
	server := newFlakyServer(t, func(_ int64, w http.ResponseWriter, _ *http.Request) {
		http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
	})
	exp := newStartedExporter(t, retryTestConfig(server.URL, time.Second, time.Hour))

	// One request is retried and the others wait in the queue.
	for range 5 {
		require.NoError(t, exp.ConsumeMetrics(t.Context(), testdata.GenerateMetricsOneMetric()))
	}
	require.Eventually(t, func() bool { return server.attempts.Load() >= 2 }, 5*time.Second, 5*time.Millisecond)

	done := make(chan error, 1)
	go func() { done <- exp.Shutdown(context.Background()) }() //nolint:usetesting // the test context is already canceled
	select {
	case err := <-done:
		assert.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("the shutdown did not complete while the backend is unavailable")
	}
}

// TestPermanentErrorsAreNotRetried verifies that the per-attempt timeout does not change the
// handling of non-retryable responses.
func TestPermanentErrorsAreNotRetried(t *testing.T) {
	server := newFlakyServer(t, func(_ int64, w http.ResponseWriter, _ *http.Request) {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
	})
	prwe := newTestPRWExporter(t, retryTestConfig(server.URL, time.Second, time.Hour))

	err := prwe.execute(t.Context(), emptyRequestBuffer(t))
	require.Error(t, err)
	assert.True(t, consumererror.IsPermanent(err))
	assert.Equal(t, int64(1), server.attempts.Load())
}

// TestParentCancelDuringBackoff verifies that canceling the context while a request waits to be
// retried interrupts it with a permanent error, and that the error is not replaced by the error
// of the last attempt, which is only done when the retries are stopped by the shutdown.
func TestParentCancelDuringBackoff(t *testing.T) {
	server := newFlakyServer(t, serviceUnavailable)
	cfg := retryTestConfig(server.URL, time.Second, time.Hour)
	cfg.BackOffConfig.InitialInterval = 300 * time.Millisecond
	cfg.BackOffConfig.MaxInterval = time.Second
	prwe := newTestPRWExporter(t, cfg)
	buf := emptyRequestBuffer(t)

	ctx, cancel := context.WithCancel(context.Background()) //nolint:usetesting // the test cancels it itself
	defer cancel()
	errCh := make(chan error, 1)
	go func() { errCh <- prwe.execute(ctx, buf) }()
	require.Eventually(t, func() bool { return server.attempts.Load() >= 1 }, 5*time.Second, 5*time.Millisecond)
	cancel()

	select {
	case err := <-errCh:
		require.Error(t, err)
		assert.ErrorIs(t, err, context.Canceled)
		assert.True(t, consumererror.IsPermanent(err))
	case <-time.After(3 * time.Second):
		t.Fatal("the retries were not interrupted by the cancellation")
	}
}

// TestRetriesDoNotLeakGoroutines verifies that every execute releases what it started, also when
// its context is never canceled, like the context of the queue consumers or of the WAL.
func TestRetriesDoNotLeakGoroutines(t *testing.T) {
	healthy := newFlakyServer(t, func(_ int64, w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	rejecting := newFlakyServer(t, func(_ int64, w http.ResponseWriter, _ *http.Request) {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
	})
	prweHealthy := newTestPRWExporter(t, retryTestConfig(healthy.URL, time.Second, time.Minute))
	prweRejecting := newTestPRWExporter(t, retryTestConfig(rejecting.URL, time.Second, time.Minute))
	buf := emptyRequestBuffer(t)

	// Warm up the connections, so that their goroutines are part of the baseline.
	require.NoError(t, prweHealthy.execute(context.Background(), buf)) //nolint:usetesting // never canceled on purpose
	require.Error(t, prweRejecting.execute(context.Background(), buf)) //nolint:usetesting // never canceled on purpose
	time.Sleep(50 * time.Millisecond)
	before := runtime.NumGoroutine()

	for range 50 {
		require.NoError(t, prweHealthy.execute(context.Background(), buf)) //nolint:usetesting // never canceled on purpose
		require.Error(t, prweRejecting.execute(context.Background(), buf)) //nolint:usetesting // never canceled on purpose
	}
	assert.Eventually(t, func() bool { return runtime.NumGoroutine() <= before+10 }, 3*time.Second, 20*time.Millisecond,
		"goroutines before: %d, now: %d", before, runtime.NumGoroutine())
}

// TestAttemptTimeoutAndHTTPTimeout verifies that an attempt is bounded by the smaller of the
// exporter timeout and the HTTP client timeout, and that a zero value of one of them leaves the other.
func TestAttemptTimeoutAndHTTPTimeout(t *testing.T) {
	for name, configure := range map[string]func(*Config){
		"http timeout shorter":  func(c *Config) { c.TimeoutSettings.Timeout = time.Minute; c.HTTP.Timeout = 100 * time.Millisecond },
		"exporter timeout zero": func(c *Config) { c.TimeoutSettings.Timeout = 0; c.HTTP.Timeout = 100 * time.Millisecond },
		"http timeout zero":     func(c *Config) { c.TimeoutSettings.Timeout = 100 * time.Millisecond; c.HTTP.Timeout = 0 },
	} {
		t.Run(name, func(t *testing.T) {
			var delivered atomic.Int64
			server := newFlakyServer(t, func(attempt int64, w http.ResponseWriter, r *http.Request) {
				if attempt <= 2 {
					hang(r)
					return
				}
				delivered.Add(1)
				w.WriteHeader(http.StatusOK)
			})
			cfg := retryTestConfig(server.URL, time.Minute, time.Minute)
			configure(cfg)
			prwe := newTestPRWExporter(t, cfg)

			start := time.Now()
			require.NoError(t, prwe.execute(t.Context(), emptyRequestBuffer(t)))
			assert.Less(t, time.Since(start), 5*time.Second)
			assert.Equal(t, int64(1), delivered.Load())
		})
	}
}

// TestMaxElapsedTimeZeroRetriesUntilShutdown verifies that max_elapsed_time 0 retries until the
// exporter shuts down.
func TestMaxElapsedTimeZeroRetriesUntilShutdown(t *testing.T) {
	server := newFlakyServer(t, serviceUnavailable)
	prwe := newTestPRWExporter(t, retryTestConfig(server.URL, 50*time.Millisecond, 0))
	buf := emptyRequestBuffer(t)

	errCh := make(chan error, 1)
	go func() { errCh <- prwe.execute(context.Background(), buf) }() //nolint:usetesting // must outlive the test context
	time.Sleep(500 * time.Millisecond)
	select {
	case err := <-errCh:
		t.Fatalf("gave up with max_elapsed_time 0: %v", err)
	default:
	}
	assert.Greater(t, server.attempts.Load(), int64(5))

	require.NoError(t, prwe.Shutdown(context.Background())) //nolint:usetesting // the test context is already canceled
	select {
	case err := <-errCh:
		require.Error(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("the retries were not interrupted by the shutdown")
	}
}

// TestQueueDisabledBlocksUntilShutdown verifies that, without the queue, the caller waits while the
// request is retried and is released by the shutdown.
func TestQueueDisabledBlocksUntilShutdown(t *testing.T) {
	server := newFlakyServer(t, serviceUnavailable)
	cfg := retryTestConfig(server.URL, time.Second, time.Hour)
	cfg.RemoteWriteQueue.Enabled = false
	exp := newStartedExporter(t, cfg)

	errCh := make(chan error, 1)
	go func() { errCh <- exp.ConsumeMetrics(context.Background(), testdata.GenerateMetricsOneMetric()) }() //nolint:usetesting // must outlive the test context
	require.Eventually(t, func() bool { return server.attempts.Load() >= 3 }, 5*time.Second, 5*time.Millisecond)
	select {
	case err := <-errCh:
		t.Fatalf("returned while the request is retried: %v", err)
	default:
	}

	require.NoError(t, exp.Shutdown(context.Background())) //nolint:usetesting // the test context is already canceled
	select {
	case err := <-errCh:
		assert.Error(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("the blocked caller was not released by the shutdown")
	}
}

// TestShutdownWhileBackendHangs verifies that the shutdown, including the flush of the queue, is
// bounded by the timeout of the attempts when the backend does not respond, and that the queued
// requests are attempted once.
func TestShutdownWhileBackendHangs(t *testing.T) {
	server := newFlakyServer(t, func(_ int64, _ http.ResponseWriter, r *http.Request) { hang(r) })
	exp := newStartedExporter(t, retryTestConfig(server.URL, 100*time.Millisecond, time.Hour))

	const queued = 5
	for range queued {
		require.NoError(t, exp.ConsumeMetrics(t.Context(), testdata.GenerateMetricsOneMetric()))
	}
	require.Eventually(t, func() bool { return server.attempts.Load() >= 1 }, 5*time.Second, 5*time.Millisecond)
	before := server.attempts.Load()

	start := time.Now()
	require.NoError(t, exp.Shutdown(context.Background())) //nolint:usetesting // the test context is already canceled
	assert.Less(t, time.Since(start), 5*time.Second)
	assert.LessOrEqual(t, server.attempts.Load()-before, int64(queued), "the queued requests must not be retried")
}

// TestShutdownFlushesQueueToHealthyBackend verifies that stopping the retries does not stop the
// delivery of the queued requests to a backend that is available.
func TestShutdownFlushesQueueToHealthyBackend(t *testing.T) {
	var delivered atomic.Int64
	server := newFlakyServer(t, func(_ int64, w http.ResponseWriter, _ *http.Request) {
		time.Sleep(20 * time.Millisecond)
		delivered.Add(1)
		w.WriteHeader(http.StatusOK)
	})
	exp := newStartedExporter(t, retryTestConfig(server.URL, time.Second, time.Hour))

	const queued = 10
	for range queued {
		require.NoError(t, exp.ConsumeMetrics(t.Context(), testdata.GenerateMetricsOneMetric()))
	}
	require.NoError(t, exp.Shutdown(context.Background())) //nolint:usetesting // the test context is already canceled
	assert.Equal(t, int64(queued), delivered.Load(), "the queued requests must be flushed on shutdown")
}

func walTestConfig(endpoint, dir string, timeout time.Duration) *Config {
	cfg := retryTestConfig(endpoint, timeout, time.Hour)
	cfg.WAL = configoptional.Some(WALConfig{Directory: dir, BufferSize: 1, TruncateFrequency: 50 * time.Millisecond})
	return cfg
}

// TestWALAttemptTimeout verifies that the requests read from the WAL are retried when attempts time out.
func TestWALAttemptTimeout(t *testing.T) {
	var delivered atomic.Int64
	server := newFlakyServer(t, func(attempt int64, w http.ResponseWriter, r *http.Request) {
		if attempt <= 2 {
			hang(r)
			return
		}
		delivered.Add(1)
		w.WriteHeader(http.StatusOK)
	})
	exp := newStartedExporter(t, walTestConfig(server.URL, t.TempDir(), 150*time.Millisecond))

	require.NoError(t, exp.ConsumeMetrics(t.Context(), testdata.GenerateMetricsOneMetric()))
	require.Eventually(t, func() bool { return delivered.Load() > 0 }, 10*time.Second, 10*time.Millisecond)
}

// TestWALShutdownWhileBackendUnavailable verifies that the shutdown does not wait for the retries of
// the requests read from the WAL, and that those requests are delivered after a restart.
func TestWALShutdownWhileBackendUnavailable(t *testing.T) {
	dir := t.TempDir()
	down := newFlakyServer(t, serviceUnavailable)
	exp, err := NewFactory().CreateMetrics(t.Context(), exportertest.NewNopSettings(metadata.Type), walTestConfig(down.URL, dir, time.Second))
	require.NoError(t, err)
	require.NoError(t, exp.Start(t.Context(), componenttest.NewNopHost()))
	require.NoError(t, exp.ConsumeMetrics(t.Context(), testdata.GenerateMetricsOneMetric()))
	require.Eventually(t, func() bool { return down.attempts.Load() >= 2 }, 5*time.Second, 5*time.Millisecond)

	start := time.Now()
	require.NoError(t, exp.Shutdown(context.Background())) //nolint:usetesting // the test context is already canceled
	assert.Less(t, time.Since(start), 5*time.Second)

	var delivered atomic.Int64
	up := newFlakyServer(t, func(_ int64, w http.ResponseWriter, _ *http.Request) {
		delivered.Add(1)
		w.WriteHeader(http.StatusOK)
	})
	newStartedExporter(t, walTestConfig(up.URL, dir, time.Second))
	require.Eventually(t, func() bool { return delivered.Load() > 0 }, 10*time.Second, 10*time.Millisecond,
		"the data in the WAL must be delivered after the restart")
}

// metricsWithSeries returns a gauge with n data points that become n time series.
func metricsWithSeries(n int) pmetric.Metrics {
	md := pmetric.NewMetrics()
	metric := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()
	metric.SetName("series")
	points := metric.SetEmptyGauge().DataPoints()
	for i := range n {
		dp := points.AppendEmpty()
		dp.Attributes().PutInt("series", int64(i))
		dp.SetDoubleValue(1)
		dp.SetTimestamp(pcommon.NewTimestampFromTime(time.Now()))
	}
	return md
}

// splitRequestsConfig returns a configuration that sends every time series in its own request, one
// request at a time, so that one batch of metrics is exported as several requests in turn.
func splitRequestsConfig(endpoint string, maxElapsedTime time.Duration) *Config {
	cfg := retryTestConfig(endpoint, time.Second, maxElapsedTime)
	cfg.RemoteWriteQueue.Enabled = false
	cfg.MaxBatchSizeBytes = 1
	parallelism := 1
	cfg.MaxBatchRequestParallelism = &parallelism
	return cfg
}

// TestMaxElapsedTimeBoundsExport verifies that max_elapsed_time bounds the export of a whole batch of
// metrics, also when the batch is split into more requests than are sent in parallel. Each of those
// requests used to be retried for max_elapsed_time in turn.
func TestMaxElapsedTimeBoundsExport(t *testing.T) {
	for _, tt := range []struct {
		name    string
		protoV2 bool
	}{
		{name: "remote write 1"},
		{name: "remote write 2", protoV2: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if tt.protoV2 {
				defer testutil.SetFeatureGateForTest(t, metadata.ExporterPrometheusremotewritexporterEnableSendingRW2FeatureGate, true)()
			}
			const (
				maxElapsedTime = 300 * time.Millisecond
				series         = 4
			)
			server := newFlakyServer(t, serviceUnavailable)
			cfg := splitRequestsConfig(server.URL, maxElapsedTime)
			if tt.protoV2 {
				cfg.RemoteWriteProtoMsg = remoteapi.WriteV2MessageType
			}
			exp := newStartedExporter(t, cfg)

			start := time.Now()
			err := exp.ConsumeMetrics(t.Context(), metricsWithSeries(series))
			elapsed := time.Since(start)

			require.Error(t, err)
			assertNonPermanentError(t, err)
			assert.ErrorContains(t, err, "requests were not sent", "the requests that were not attempted must be reported")
			assert.Less(t, elapsed, 2*maxElapsedTime, "max_elapsed_time must bound the export of the batch, not of every request")
			assert.Greater(t, server.attempts.Load(), int64(3), "the first request must be retried")
		})
	}
}

// TestExportReportsUnsentRequests verifies that an export whose context is done before all the
// requests were sent returns an error, instead of returning nil for the requests it never attempted.
func TestExportReportsUnsentRequests(t *testing.T) {
	server := newFlakyServer(t, func(_ int64, w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	prwe := newTestPRWExporter(t, splitRequestsConfig(server.URL, time.Hour))

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	err := prwe.export(ctx, []*prompb.WriteRequest{{}, {}, {}})

	require.Error(t, err)
	assert.ErrorContains(t, err, "3 of 3 requests were not sent")
	assert.ErrorIs(t, err, context.Canceled)
	assert.Zero(t, server.attempts.Load())
}

// TestShutdownDeadlineBoundsQueueFlush verifies that the deadline of the shutdown context bounds the
// flush of the queue: once it passed, the queued requests are no longer attempted, instead of every
// request being attempted for the duration of the timeout.
func TestShutdownDeadlineBoundsQueueFlush(t *testing.T) {
	const (
		timeout  = time.Second
		deadline = 300 * time.Millisecond
		queued   = 5
	)
	server := newFlakyServer(t, func(_ int64, _ http.ResponseWriter, r *http.Request) { hang(r) })
	exp := newStartedExporter(t, retryTestConfig(server.URL, timeout, time.Hour))

	for range queued {
		require.NoError(t, exp.ConsumeMetrics(t.Context(), testdata.GenerateMetricsOneMetric()))
	}
	require.Eventually(t, func() bool { return server.attempts.Load() >= 1 }, 5*time.Second, 5*time.Millisecond)

	ctx, cancel := context.WithTimeout(t.Context(), deadline)
	defer cancel()
	start := time.Now()
	require.NoError(t, exp.Shutdown(ctx))
	elapsed := time.Since(start)
	assert.GreaterOrEqual(t, elapsed, deadline/4, "the in-flight request must be given until the deadline")
	assert.Less(t, elapsed, (queued-1)*timeout, "the queued requests must not be attempted after the deadline of the shutdown")
}
