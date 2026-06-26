// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package notify

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.uber.org/zap"
)

const testScope = "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awss3exporter"

// testTelemetry bundles a TelemetrySettings and a ManualReader so tests can
// round-trip metrics without mocking the SDK.
type testTelemetry struct {
	settings component.TelemetrySettings
	reader   *metric.ManualReader
}

func newTestTelemetry(t *testing.T) *testTelemetry {
	t.Helper()
	reader := metric.NewManualReader()
	mp := metric.NewMeterProvider(metric.WithReader(reader))
	ts := componenttest.NewNopTelemetrySettings()
	ts.MeterProvider = mp
	ts.Logger = zap.NewNop()
	return &testTelemetry{settings: ts, reader: reader}
}

// collect returns the single ScopeMetrics produced by the notifier. The SDK
// exposes one entry because the notifier creates a single Meter.
func (tt *testTelemetry) collect(t *testing.T) metricdata.ScopeMetrics {
	t.Helper()
	var rm metricdata.ResourceMetrics
	require.NoError(t, tt.reader.Collect(context.Background(), &rm))
	for _, sm := range rm.ScopeMetrics {
		if sm.Scope.Name == testScope {
			return sm
		}
	}
	if len(rm.ScopeMetrics) == 1 {
		return rm.ScopeMetrics[0]
	}
	t.Fatalf("expected exactly one ScopeMetrics for %q, got %d", testScope, len(rm.ScopeMetrics))
	return metricdata.ScopeMetrics{}
}

func sumValue(t *testing.T, sm metricdata.ScopeMetrics, name, attrKey, attrVal string) int64 {
	t.Helper()
	for _, m := range sm.Metrics {
		if m.Name != name {
			continue
		}
		sum, ok := m.Data.(metricdata.Sum[int64])
		if !ok {
			t.Fatalf("metric %q is not Sum[int64]", name)
		}
		for _, dp := range sum.DataPoints {
			if v, present := dp.Attributes.Value(attribute.Key(attrKey)); present && v.AsString() == attrVal {
				return dp.Value
			}
		}
	}
	return 0
}

func histogramSamples(t *testing.T, sm metricdata.ScopeMetrics, name, attrKey, attrVal string) uint64 {
	t.Helper()
	for _, m := range sm.Metrics {
		if m.Name != name {
			continue
		}
		h, ok := m.Data.(metricdata.Histogram[float64])
		if !ok {
			t.Fatalf("metric %q is not Histogram[float64]", name)
		}
		for _, dp := range h.DataPoints {
			if v, present := dp.Attributes.Value(attribute.Key(attrKey)); present && v.AsString() == attrVal {
				return dp.Count
			}
		}
	}
	return 0
}

// waitFor polls pred with a short sleep until it returns true or the
// deadline expires. Preferred over fixed sleeps for assertions about
// eventually-consistent state like "N requests have arrived".
func waitFor(t *testing.T, timeout time.Duration, pred func() bool) bool {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		if pred() {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(5 * time.Millisecond)
	}
}

type captureServer struct {
	mu       sync.Mutex
	requests [][]byte
	headers  []http.Header
}

func (s *captureServer) record(body []byte, headers http.Header) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requests = append(s.requests, body)
	s.headers = append(s.headers, headers.Clone())
}

func (s *captureServer) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.requests)
}

func (s *captureServer) allRecords(t *testing.T) []s3Record {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []s3Record
	for _, body := range s.requests {
		var env s3Envelope
		require.NoError(t, json.Unmarshal(body, &env))
		out = append(out, env.Records...)
	}
	return out
}

// newTestNotifier returns a live httpNotifier wired to the given handler,
// with short backoffs so retry-heavy tests stay fast.
func newTestNotifier(t *testing.T, handler http.Handler, tune func(c *Config)) (*httpNotifier, *testTelemetry, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	cfg := NewDefaultConfig()
	cfg.Endpoint = srv.URL
	cfg.InitialBackoff = time.Millisecond
	cfg.MaxBackoff = 10 * time.Millisecond
	if tune != nil {
		tune(&cfg)
	}

	tt := newTestTelemetry(t)
	n, err := New(cfg, testScope, tt.settings, componenttest.NewNopHost(), zap.NewNop())
	require.NoError(t, err)

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = n.Shutdown(ctx)
	})

	hn, ok := n.(*httpNotifier)
	require.True(t, ok, "expected live httpNotifier")
	return hn, tt, srv
}

func TestNoop(t *testing.T) {
	t.Parallel()

	cfg := NewDefaultConfig() // Endpoint empty
	ts := componenttest.NewNopTelemetrySettings()
	n, err := New(cfg, testScope, ts, componenttest.NewNopHost(), zap.NewNop())
	require.NoError(t, err)

	_, ok := n.(noopNotifier)
	assert.True(t, ok, "expected noopNotifier for empty endpoint")
	assert.False(t, n.Enqueue(t.Context(), Event{Bucket: "b", Key: "k", Size: 1}))
	assert.NoError(t, n.Shutdown(t.Context()))
}

func TestHappyPath(t *testing.T) {
	t.Parallel()

	cs := &captureServer{}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		cs.record(body, r.Header)
		w.WriteHeader(http.StatusOK)
	})

	n, tt, _ := newTestNotifier(t, handler, func(c *Config) {
		c.MaxRecordsPerPost = 10
		c.Workers = 2
		c.Headers.Set("Authorization", configopaque.String("Bearer test-token"))
	})

	const N = 50
	for i := 0; i < N; i++ {
		assert.True(t, n.Enqueue(t.Context(), Event{
			Bucket: "my-bucket",
			Key:    "a+b/obj.json",
			Size:   int64(i + 1),
		}))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, n.Shutdown(ctx))

	records := cs.allRecords(t)
	assert.Len(t, records, N, "every enqueued event must reach the receiver")
	for _, rec := range records {
		assert.Equal(t, "my-bucket", rec.S3.Bucket.Name)
		assert.Equal(t, "a%2Bb%2Fobj.json", rec.S3.Object.Key)
	}

	// Content-Type and Authorization header assertions.
	require.Positive(t, cs.count())
	cs.mu.Lock()
	firstHeaders := cs.headers[0]
	cs.mu.Unlock()
	assert.Equal(t, "application/json", firstHeaders.Get("Content-Type"))
	assert.Equal(t, "Bearer test-token", firstHeaders.Get("Authorization"))

	// Sanity: no drops.
	sm := tt.collect(t)
	assert.EqualValues(t, N, sumValue(t, sm, MetricNameSent, attrOutcome, outcomeSuccess))
	assert.EqualValues(t, 0, sumValue(t, sm, MetricNameDropped, attrReason, ReasonQueueFull))
	assert.EqualValues(t, 0, sumValue(t, sm, MetricNameDropped, attrReason, ReasonShutdown))
}

func TestBatching(t *testing.T) {
	t.Parallel()

	cs := &captureServer{}
	// Hold the first request for a beat so events pile up in the queue;
	// the single worker then greedy-drains up to MaxRecordsPerPost on the
	// next iteration.
	started := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		once.Do(func() {
			close(started)
			<-release
		})
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		cs.record(body, r.Header)
		w.WriteHeader(http.StatusOK)
	})

	n, _, _ := newTestNotifier(t, handler, func(c *Config) {
		c.Workers = 1
		c.MaxRecordsPerPost = 100
		c.QueueSize = 1000
	})

	// Enqueue one event to trigger the first handler invocation.
	assert.True(t, n.Enqueue(t.Context(), Event{Bucket: "b", Key: "first", Size: 1}))
	<-started

	// Queue 249 more while the worker is blocked; all should land in ch.
	for i := 1; i < 250; i++ {
		assert.True(t, n.Enqueue(t.Context(), Event{Bucket: "b", Key: "more", Size: int64(i)}))
	}
	close(release)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, n.Shutdown(ctx))

	records := cs.allRecords(t)
	assert.Len(t, records, 250)

	posts := cs.count()
	assert.GreaterOrEqual(t, posts, 3, "250 events at 100/post requires at least 3 POSTs")

	// Each POST body must carry <= 100 records.
	cs.mu.Lock()
	for i, body := range cs.requests {
		var env s3Envelope
		require.NoError(t, json.Unmarshal(body, &env))
		assert.LessOrEqual(t, len(env.Records), 100, "POST %d carried %d records", i, len(env.Records))
	}
	cs.mu.Unlock()
}

func TestRetriesSucceed(t *testing.T) {
	t.Parallel()

	cs := &captureServer{}
	var attempts atomic.Int32
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		cs.record(body, r.Header)
		if attempts.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	n, tt, _ := newTestNotifier(t, handler, func(c *Config) {
		c.MaxAttempts = 3
		c.Workers = 1
	})

	assert.True(t, n.Enqueue(t.Context(), Event{Bucket: "b", Key: "k", Size: 1}))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, n.Shutdown(ctx))

	assert.EqualValues(t, 2, attempts.Load(), "exactly two HTTP attempts (fail + succeed)")

	sm := tt.collect(t)
	assert.EqualValues(t, 1, sumValue(t, sm, MetricNameSent, attrOutcome, outcomeSuccess))
	assert.EqualValues(t, 0, sumValue(t, sm, MetricNameDropped, attrReason, ReasonRetriesExhausted))
	assert.EqualValues(t, 1, histogramSamples(t, sm, MetricNameSendDuration, attrStatusClass, StatusClass5xx))
	assert.EqualValues(t, 1, histogramSamples(t, sm, MetricNameSendDuration, attrStatusClass, StatusClass2xx))
}

func TestRetriesExhausted(t *testing.T) {
	t.Parallel()

	cs := &captureServer{}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		cs.record(body, r.Header)
		w.WriteHeader(http.StatusServiceUnavailable)
	})

	n, tt, _ := newTestNotifier(t, handler, func(c *Config) {
		c.MaxAttempts = 3
		c.Workers = 1
	})

	assert.True(t, n.Enqueue(t.Context(), Event{Bucket: "b", Key: "k", Size: 1}))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, n.Shutdown(ctx))

	assert.Equal(t, 3, cs.count(), "all three attempts reached the server")

	sm := tt.collect(t)
	assert.EqualValues(t, 0, sumValue(t, sm, MetricNameSent, attrOutcome, outcomeSuccess))
	assert.EqualValues(t, 1, sumValue(t, sm, MetricNameDropped, attrReason, ReasonRetriesExhausted))
	assert.EqualValues(t, 3, histogramSamples(t, sm, MetricNameSendDuration, attrStatusClass, StatusClass5xx))
}

func TestPermanent4xx(t *testing.T) {
	t.Parallel()

	cs := &captureServer{}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		cs.record(body, r.Header)
		w.WriteHeader(http.StatusBadRequest)
	})

	n, tt, _ := newTestNotifier(t, handler, func(c *Config) {
		c.MaxAttempts = 3
		c.Workers = 1
	})

	assert.True(t, n.Enqueue(t.Context(), Event{Bucket: "b", Key: "k", Size: 1}))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, n.Shutdown(ctx))

	assert.Equal(t, 1, cs.count(), "4xx must not be retried")

	sm := tt.collect(t)
	assert.EqualValues(t, 0, sumValue(t, sm, MetricNameSent, attrOutcome, outcomeSuccess))
	assert.EqualValues(t, 1, sumValue(t, sm, MetricNameDropped, attrReason, ReasonPermanent4xx))
	assert.EqualValues(t, 1, histogramSamples(t, sm, MetricNameSendDuration, attrStatusClass, StatusClass4xx))
}

func TestQueueFull(t *testing.T) {
	t.Parallel()

	released := make(chan struct{})
	var inFlight atomic.Int32
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		inFlight.Add(1)
		<-released
		w.WriteHeader(http.StatusOK)
	})

	n, tt, _ := newTestNotifier(t, handler, func(c *Config) {
		c.QueueSize = 2
		c.Workers = 1
		c.MaxRecordsPerPost = 1
	})

	// Wait until the single worker has claimed the first event and is
	// blocked in the handler; otherwise the in-flight worker may still be
	// draining and the queue never actually fills.
	assert.True(t, n.Enqueue(t.Context(), Event{Bucket: "b", Key: "k0", Size: 0}))
	require.True(t, waitFor(t, 2*time.Second, func() bool {
		return inFlight.Load() >= 1
	}), "first handler invocation did not arrive")

	// The worker has pulled k0 and is stuck; the queue can now accept up to
	// QueueSize events before Enqueue starts dropping.
	accepted := 0
	dropped := 0
	for i := 1; i < 11; i++ {
		if n.Enqueue(t.Context(), Event{Bucket: "b", Key: "k", Size: int64(i)}) {
			accepted++
		} else {
			dropped++
		}
	}

	// Release the server so the notifier can drain cleanly.
	close(released)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, n.Shutdown(ctx))

	// Concurrency means precise counts are not deterministic, but the
	// weaker invariants below are.
	assert.Equal(t, 10, accepted+dropped, "every Enqueue returns a decision")
	assert.Positive(t, dropped, "at least one Enqueue must have seen the full queue")

	sm := tt.collect(t)
	queueFullDrops := sumValue(t, sm, MetricNameDropped, attrReason, ReasonQueueFull)
	assert.Positive(t, queueFullDrops, "at least one drop recorded with reason=queue_full")
	assert.EqualValues(t, int64(dropped), queueFullDrops,
		"every Enqueue-returned-false must correspond to a queue_full drop")
}

func TestShutdownDrain(t *testing.T) {
	t.Parallel()

	cs := &captureServer{}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		cs.record(body, r.Header)
		w.WriteHeader(http.StatusOK)
	})

	n, tt, _ := newTestNotifier(t, handler, func(c *Config) {
		c.MaxRecordsPerPost = 10
		c.Workers = 2
	})

	const N = 100
	for i := 0; i < N; i++ {
		require.True(t, n.Enqueue(t.Context(), Event{Bucket: "b", Key: "k", Size: int64(i)}))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	require.NoError(t, n.Shutdown(ctx))

	assert.Len(t, cs.allRecords(t), N)
	sm := tt.collect(t)
	assert.EqualValues(t, N, sumValue(t, sm, MetricNameSent, attrOutcome, outcomeSuccess))
}

func TestShutdownDeadline(t *testing.T) {
	t.Parallel()

	// Server stalls every request until either the client goes away (its
	// context is cancelled) or we explicitly release it in t.Cleanup. This
	// ensures the handler goroutine always exits, which is required for
	// httptest.Server.Close to return without leaking a goroutine.
	release := make(chan struct{})
	t.Cleanup(func() {
		select {
		case <-release:
		default:
			close(release)
		}
	})
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-release:
		}
		// Best-effort response; the client has already given up.
		w.WriteHeader(http.StatusOK)
	})

	n, tt, _ := newTestNotifier(t, handler, func(c *Config) {
		c.Workers = 1
		c.MaxRecordsPerPost = 1
		c.QueueSize = 1000
		// Give the HTTP client enough headroom that the request doesn't
		// trigger its own timeout — we're exercising Shutdown's deadline,
		// not ClientConfig.Timeout.
		c.Timeout = 30 * time.Second
	})

	const N = 100
	for i := 0; i < N; i++ {
		require.True(t, n.Enqueue(t.Context(), Event{Bucket: "b", Key: "k", Size: int64(i)}))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	require.NoError(t, n.Shutdown(ctx))
	elapsed := time.Since(start)
	assert.Less(t, elapsed, 2*time.Second, "Shutdown must return promptly after its deadline")

	// Release any still-pending handler goroutines so httptest.Server.Close
	// in t.Cleanup does not block.
	close(release)

	sm := tt.collect(t)
	sent := sumValue(t, sm, MetricNameSent, attrOutcome, outcomeSuccess)
	shutdownDrops := sumValue(t, sm, MetricNameDropped, attrReason, ReasonShutdown)
	retriesDrops := sumValue(t, sm, MetricNameDropped, attrReason, ReasonRetriesExhausted)
	queueFull := sumValue(t, sm, MetricNameDropped, attrReason, ReasonQueueFull)
	// Cancelling an in-flight POST surfaces as a retriable network error;
	// postBatch must classify that as reason=shutdown (not retries_exhausted)
	// so operators can distinguish a genuine endpoint outage from an expected
	// deadline-clipped drain.
	assert.EqualValues(t, 0, queueFull)
	assert.EqualValues(t, 0, retriesDrops,
		"shutdown-cancelled attempts must not be classified as retries_exhausted")
	assert.EqualValues(t, N, sent+shutdownDrops,
		"sent + shutdown drops must cover every enqueued event when shutdown was the cause")
}

func TestURLEncoding(t *testing.T) {
	t.Parallel()

	cs := &captureServer{}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		cs.record(body, r.Header)
		w.WriteHeader(http.StatusOK)
	})

	n, _, _ := newTestNotifier(t, handler, nil)
	require.True(t, n.Enqueue(t.Context(), Event{
		Bucket: "b",
		Key:    "raw/plus+char file.json",
		Size:   10,
	}))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, n.Shutdown(ctx))

	records := cs.allRecords(t)
	require.Len(t, records, 1)
	assert.Equal(t, "raw%2Fplus%2Bchar+file.json", records[0].S3.Object.Key)
}

func TestConcurrentEnqueueDuringShutdown(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	n, _, _ := newTestNotifier(t, handler, func(c *Config) {
		c.QueueSize = 10
		c.Workers = 2
	})

	const producers = 8
	var wg sync.WaitGroup
	stop := make(chan struct{})
	for i := 0; i < producers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					n.Enqueue(context.Background(), Event{Bucket: "b", Key: "k", Size: int64(id)})
				}
			}
		}(i)
	}

	time.Sleep(10 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, n.Shutdown(ctx))

	close(stop)
	wg.Wait()
	// goleak (in TestMain at the package) catches leaked goroutines; no
	// explicit assertion needed here beyond "Shutdown returned cleanly".
}
