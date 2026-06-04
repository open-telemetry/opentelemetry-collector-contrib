// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package notify delivers HTTP webhook notifications after each successful
// S3 upload performed by the awss3exporter.
//
// The package is kept free of references to the exporter root package and to
// the upload sub-package so it can be wired via thin adapters without
// introducing import cycles.
package notify // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awss3exporter/internal/notify"

import (
	"bytes"
	"context"
	"fmt"
	"io"
	mathrand "math/rand/v2"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"
)

// Event is the minimum tuple carried from the upload manager to the notifier.
// Key is stored in raw (un-escaped) form; the notifier is responsible for
// URL-encoding the key at marshal time.
type Event struct {
	// Bucket is the S3 bucket name (sent verbatim).
	Bucket string
	// Key is the raw, un-escaped object key.
	Key string
	// Size is the post-compression byte length that was written to S3.
	Size int64
}

// Notifier is the contract consumed by the upload manager.
//
// Enqueue is non-blocking. It returns true when the event was accepted into
// the queue and false when it was dropped (queue full, shutdown in progress,
// or caller context cancelled). Callers are expected to ignore the return
// value; drop accounting is recorded via the notifier's own metrics.
//
// Shutdown stops accepting new events, drains queued work within the deadline
// of the supplied context, and returns. It is safe to call multiple times.
type Notifier interface {
	Enqueue(ctx context.Context, e Event) bool
	Shutdown(ctx context.Context) error
}

// noopNotifier is the inactive implementation returned when the feature is
// disabled (Config.Endpoint == "").
type noopNotifier struct{}

// Enqueue returns false to signal "not accepted". Callers ignore the return
// value; this choice preserves the invariant that Enqueue never performs I/O.
func (noopNotifier) Enqueue(_ context.Context, _ Event) bool { return false }

// Shutdown is a no-op for the disabled path.
func (noopNotifier) Shutdown(_ context.Context) error { return nil }

// NewNoop returns a Notifier that accepts nothing and does nothing. It exists
// to keep test wiring concise and to prevent callers from using a nil
// Notifier.
func NewNoop() Notifier { return noopNotifier{} }

// httpNotifier is the live implementation: bounded in-memory queue, shared
// worker pool, size-triggered batching, per-batch retry with exponential
// backoff, and graceful drain on shutdown.
type httpNotifier struct {
	cfg    Config
	client *http.Client
	logger *zap.Logger
	instr  *instruments

	ch     chan Event
	stopCh chan struct{}

	// accepting guards the producer path: once false, Enqueue immediately
	// records a shutdown drop and returns false.
	accepting atomic.Bool

	// shutdownCtx is cancelled either by Shutdown's ctx deadline or during
	// the explicit cancel in Shutdown. Workers' in-flight POSTs and retry
	// sleeps observe it so they unwind promptly.
	shutdownCtx    context.Context
	cancelShutdown context.CancelFunc

	wg sync.WaitGroup

	// rand is indirected so tests can swap in a deterministic source. The
	// default calls math/rand/v2's per-goroutine safe Float64. Each backoff
	// call draws fresh jitter so concurrent workers do not retry in lockstep.
	rand func() float64
}

// New builds a Notifier from the supplied configuration.
//
// When cfg.Endpoint is empty the feature is disabled and a noop Notifier is
// returned. Otherwise a live httpNotifier is started with the configured
// queue, workers, and retry policy.
func New(cfg Config, scopeName string, telemetry component.TelemetrySettings, host component.Host, logger *zap.Logger) (Notifier, error) {
	if cfg.Endpoint == "" {
		return noopNotifier{}, nil
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	client, err := cfg.ToClient(context.Background(), host.GetExtensions(), telemetry)
	if err != nil {
		return nil, fmt.Errorf("notifications: build http client: %w", err)
	}

	instr, err := newInstruments(telemetry.MeterProvider.Meter(scopeName))
	if err != nil {
		return nil, fmt.Errorf("notifications: build instruments: %w", err)
	}

	shutdownCtx, cancelShutdown := context.WithCancel(context.Background())

	n := &httpNotifier{
		cfg:            cfg,
		client:         client,
		logger:         logger,
		instr:          instr,
		ch:             make(chan Event, cfg.QueueSize),
		stopCh:         make(chan struct{}),
		shutdownCtx:    shutdownCtx,
		cancelShutdown: cancelShutdown,
		rand:           func() float64 { return mathrand.Float64() },
	}
	n.accepting.Store(true)

	// Pre-increment the wait group before any goroutines are launched so
	// Shutdown cannot observe a zero-count wait group while workers are
	// still being scheduled.
	n.wg.Add(cfg.Workers)
	for i := 0; i < cfg.Workers; i++ {
		go n.workerLoop()
	}

	return n, nil
}

// Enqueue attempts to place e on the queue without blocking.
//
// Semantics, in priority order:
//  1. If the producer's ctx is already cancelled, return false without
//     counting — the caller is going away, not the notifier.
//  2. If Shutdown has been called (accepting == false), record a shutdown
//     drop and return false.
//  3. Non-blocking channel send; on a full queue record a queue_full drop
//     and return false. Otherwise return true.
//
// This ordering ensures a slow webhook cannot stall an S3 upload and that
// drops are attributed to the correct cause.
func (n *httpNotifier) Enqueue(ctx context.Context, e Event) bool {
	if ctx != nil && ctx.Err() != nil {
		return false
	}
	if !n.accepting.Load() {
		n.instr.recordDropped(contextOrBackground(ctx), 1, ReasonShutdown)
		return false
	}
	select {
	case n.ch <- e:
		return true
	default:
		n.instr.recordDropped(contextOrBackground(ctx), 1, ReasonQueueFull)
		return false
	}
}

// Shutdown stops the notifier, drains what it can within ctx's deadline, and
// accounts for any leftover queued events as shutdown drops.
//
// Idempotent: subsequent calls short-circuit to nil.
func (n *httpNotifier) Shutdown(ctx context.Context) error {
	// Flip accepting false BEFORE closing stopCh so Enqueue cannot race with
	// workers entering drain mode — an Enqueue that observes
	// accepting == true may still win a non-blocking send, and that event
	// will be picked up by either a worker or the final sweep below.
	if !n.accepting.CompareAndSwap(true, false) {
		return nil
	}
	close(n.stopCh)

	done := make(chan struct{})
	go func() {
		n.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
		// Unblock in-flight POSTs and retry sleeps; workers exit via the
		// context cancellation and the drain loop's shutdownCtx check.
		n.cancelShutdown()
		<-done
	}

	// At this point no worker is reading n.ch. Account for any leftovers;
	// these are events that were enqueued after the last drain attempt in
	// a worker (possible because accepting briefly overlaps with close, but
	// the late arrivals cannot produce double counting — see comment in
	// (*httpNotifier).drain).
	for {
		select {
		case <-n.ch:
			n.instr.recordDropped(context.Background(), 1, ReasonShutdown)
		default:
			// Ensure the internal context is torn down even on the fast path
			// where workers exited cleanly and we never hit ctx.Done.
			n.cancelShutdown()
			return nil
		}
	}
}

// workerLoop is one of cfg.Workers goroutines draining n.ch. Each iteration
// pulls a single event, opportunistically greedy-drains up to
// MaxRecordsPerPost, and flushes the batch via postBatch. There is no
// batching timer: once a worker pops an event it flushes immediately, which
// means deliveries are size-triggered only.
func (n *httpNotifier) workerLoop() {
	defer n.wg.Done()
	for {
		select {
		case <-n.stopCh:
			n.drain()
			return
		case first, ok := <-n.ch:
			if !ok {
				// n.ch is never closed; this branch exists only to guard
				// against a future refactor that decides otherwise.
				return
			}
			batch := make([]Event, 0, n.cfg.MaxRecordsPerPost)
			batch = append(batch, first)
			for len(batch) < n.cfg.MaxRecordsPerPost {
				select {
				case e := <-n.ch:
					batch = append(batch, e)
				default:
					goto flush
				}
			}
		flush:
			n.postBatch(n.shutdownCtx, batch)
		}
	}
}

// drain is invoked after stopCh has been closed. It keeps pulling batches
// non-blockingly until the channel is empty, honoring shutdownCtx so the
// overall deadline still applies.
//
// Counting rule: a worker counts shutdown drops only for batches it has
// already pulled from n.ch. Events still sitting in n.ch after every worker
// exits are counted by Shutdown's final sweep — there is exactly one reader
// per residual event, so no double counting is possible.
func (n *httpNotifier) drain() {
	for {
		batch := make([]Event, 0, n.cfg.MaxRecordsPerPost)
		for len(batch) < n.cfg.MaxRecordsPerPost {
			select {
			case e := <-n.ch:
				batch = append(batch, e)
			default:
				goto checkEmpty
			}
		}
	checkEmpty:
		if len(batch) == 0 {
			return
		}
		if n.shutdownCtx.Err() != nil {
			// Shutdown deadline has fired: count the already-popped events
			// as shutdown drops and stop draining.
			n.instr.recordDropped(context.Background(), int64(len(batch)), ReasonShutdown)
			return
		}
		n.postBatch(n.shutdownCtx, batch)
	}
}

// postBatch runs the outer retry loop for one batch. It records exactly one
// terminal metric event per batch (either recordSent on success or
// recordDropped with the appropriate reason on failure) and one duration
// sample per HTTP attempt.
func (n *httpNotifier) postBatch(ctx context.Context, batch []Event) {
	body, err := marshalBatch(batch, time.Now())
	if err != nil {
		// Our own types are guaranteed-marshalable, so this is unreachable
		// short of runtime corruption. Surface as a drop rather than a
		// panic so the notifier stays alive.
		n.logger.Warn("notifications: marshal batch failed", zap.Error(err))
		n.instr.recordDropped(context.Background(), int64(len(batch)), ReasonRetriesExhausted)
		return
	}

	for attempt := 0; attempt < n.cfg.MaxAttempts; attempt++ {
		if ctx.Err() != nil {
			n.instr.recordDropped(context.Background(), int64(len(batch)), ReasonShutdown)
			return
		}

		statusClass, permanent, retriable, err := n.doOnePost(ctx, body)
		switch {
		case err == nil:
			n.instr.recordSent(ctx, int64(len(batch)))
			return
		case permanent:
			n.logger.Warn("notifications: permanent failure, dropping batch",
				zap.String("status_class", statusClass),
				zap.String("first_bucket", batch[0].Bucket),
				zap.String("first_key", batch[0].Key),
				zap.Int("records", len(batch)),
				zap.Error(err),
			)
			n.instr.recordDropped(ctx, int64(len(batch)), ReasonPermanent4xx)
			return
		case !retriable:
			// Defensive: doOnePost always classifies outcomes as either
			// permanent or retriable.
			n.instr.recordDropped(ctx, int64(len(batch)), ReasonRetriesExhausted)
			return
		}

		// Retriable. If shutdown cancelled the attempt mid-flight, the drop
		// belongs to shutdown rather than retries_exhausted — otherwise
		// operators would see phantom retry exhaustion every time a
		// Shutdown deadline clips a batch.
		if ctx.Err() != nil {
			n.instr.recordDropped(context.Background(), int64(len(batch)), ReasonShutdown)
			return
		}

		// Genuine retriable failure. If this was the last attempt, give up;
		// otherwise sleep under shutdown-aware cancellation before retrying.
		if attempt+1 == n.cfg.MaxAttempts {
			n.instr.recordDropped(ctx, int64(len(batch)), ReasonRetriesExhausted)
			return
		}
		sleep := n.backoff(attempt)
		timer := time.NewTimer(sleep)
		select {
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()
			n.instr.recordDropped(context.Background(), int64(len(batch)), ReasonShutdown)
			return
		}
	}
}

// doOnePost performs one HTTP attempt. It always records a duration sample
// before returning so the histogram reflects every attempt, including
// network errors.
//
// Returns:
//   - statusClass: one of StatusClass2xx, StatusClass4xx, StatusClass5xx,
//     StatusClassNetworkError.
//   - permanent: true when the attempt hit a 4xx and must not be retried.
//   - retriable: true when the attempt may be retried (network error or 5xx).
//   - err: nil on 2xx; otherwise a descriptive error.
func (n *httpNotifier) doOnePost(parentCtx context.Context, body []byte) (statusClass string, permanent, retriable bool, err error) {
	attemptCtx, cancel := context.WithTimeout(parentCtx, n.cfg.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(attemptCtx, http.MethodPost, n.cfg.Endpoint, bytes.NewReader(body))
	if err != nil {
		// Only possible if Endpoint was mutated post-Validate. Treat as
		// retriable network-class failure.
		return StatusClassNetworkError, false, true, err
	}
	req.Header.Set("Content-Type", "application/json")

	start := time.Now()
	resp, err := n.client.Do(req)
	elapsed := time.Since(start)
	if err != nil {
		n.instr.recordDuration(parentCtx, elapsed, StatusClassNetworkError)
		return StatusClassNetworkError, false, true, err
	}
	defer func() { _ = resp.Body.Close() }()
	// Drain to allow connection reuse.
	_, _ = io.Copy(io.Discard, resp.Body)

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		n.instr.recordDuration(parentCtx, elapsed, StatusClass2xx)
		return StatusClass2xx, false, false, nil
	case resp.StatusCode >= 400 && resp.StatusCode < 500:
		n.instr.recordDuration(parentCtx, elapsed, StatusClass4xx)
		return StatusClass4xx, true, false, fmt.Errorf("http %d", resp.StatusCode)
	case resp.StatusCode >= 500 && resp.StatusCode < 600:
		n.instr.recordDuration(parentCtx, elapsed, StatusClass5xx)
		return StatusClass5xx, false, true, fmt.Errorf("http %d", resp.StatusCode)
	default:
		// 1xx/3xx: unexpected for a JSON POST. Treat as retriable and bucket
		// into 5xx so the histogram signal stays meaningful.
		n.instr.recordDuration(parentCtx, elapsed, StatusClass5xx)
		return StatusClass5xx, false, true, fmt.Errorf("http %d", resp.StatusCode)
	}
}

// backoff computes the delay before the next attempt.
//
// base doubles with each attempt up to MaxBackoff; jitter is sampled fresh
// per call in [0.5, 1.5) so two workers retrying simultaneously do not
// thunder-herd.
func (n *httpNotifier) backoff(attempt int) time.Duration {
	base := n.cfg.InitialBackoff << attempt
	if base <= 0 || base > n.cfg.MaxBackoff {
		// <= 0 guards against the shift overflowing for large attempt values,
		// which can happen if MaxAttempts is ever configured very large.
		base = n.cfg.MaxBackoff
	}
	jitter := 0.5 + n.rand() // [0.5, 1.5)
	return time.Duration(float64(base) * jitter)
}

// contextOrBackground returns ctx if non-nil, otherwise context.Background().
// Kept private to this package so callers can safely pass a possibly-nil ctx.
func contextOrBackground(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}
