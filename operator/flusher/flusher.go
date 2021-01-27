// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package flusher

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	backoff "github.com/cenkalti/backoff/v4"
	"go.uber.org/zap"
	"golang.org/x/sync/semaphore"
)

// These are vars so they can be overridden in tests
var maxRetryInterval = time.Minute
var maxElapsedTime = time.Hour

// Config holds the configuration to build a new flusher
type Config struct {
	// MaxConcurrent is the maximum number of goroutines flushing entries concurrently.
	// Defaults to 16.
	MaxConcurrent int `json:"max_concurrent" yaml:"max_concurrent"`

	// TODO configurable retry
}

// NewConfig creates a new default flusher config
func NewConfig() Config {
	return Config{
		MaxConcurrent: 16,
	}
}

// Build uses a Config to build a new Flusher
func (c *Config) Build(logger *zap.SugaredLogger) *Flusher {
	maxConcurrent := c.MaxConcurrent
	if maxConcurrent == 0 {
		maxConcurrent = 16
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Flusher{
		ctx:           ctx,
		cancel:        cancel,
		sem:           semaphore.NewWeighted(int64(maxConcurrent)),
		SugaredLogger: logger,
	}
}

// Flusher is used to flush entries from a buffer concurrently. It handles max concurrency,
// retry behavior, and cancellation.
type Flusher struct {
	ctx            context.Context
	cancel         context.CancelFunc
	sem            *semaphore.Weighted
	wg             sync.WaitGroup
	chunkIDCounter uint64
	*zap.SugaredLogger
}

// FlushFunc is any function that flushes
type FlushFunc func(context.Context) error

// Do executes the flusher function in a goroutine
func (f *Flusher) Do(flush FlushFunc) {
	// Wait until we have free flusher goroutines
	if err := f.sem.Acquire(f.ctx, 1); err != nil {
		// Context cancelled
		return
	}

	// Start a new flusher goroutine
	f.wg.Add(1)
	go func() {
		defer f.wg.Done()
		defer f.sem.Release(1)
		f.flushWithRetry(f.ctx, flush)
	}()
}

// Stop cancels all the in-progress flushers and waits until they have returned
func (f *Flusher) Stop() {
	f.cancel()
	f.wg.Wait()
}

// flushWithRetry will continue trying to call flushFunc with the entries passed
// in until either flushFunc returns no error or the context is cancelled. It will only
// return an error in the case that the context was cancelled. If no error was returned,
// it is safe to mark the entries in the buffer as flushed.
func (f *Flusher) flushWithRetry(ctx context.Context, flush FlushFunc) {
	chunkID := f.nextChunkID()
	b := newExponentialBackoff()
	for {
		err := flush(ctx)
		if err == nil {
			return
		}

		waitTime := b.NextBackOff()
		if waitTime == b.Stop {
			f.Errorw("Reached max backoff time during chunk flush retry. Dropping logs in chunk", "chunk_id", chunkID)
			return
		}

		// Only log the error if the context hasn't been canceled
		// This protects from flooding the logs with "context canceled" messages on clean shutdown
		select {
		case <-ctx.Done():
			return
		default:
			f.Warnw("Failed flushing chunk. Waiting before retry", "error", err, "wait_time", waitTime)
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(waitTime):
		}
	}
}

func (f *Flusher) nextChunkID() uint64 {
	return atomic.AddUint64(&f.chunkIDCounter, 1)
}

// newExponentialBackoff returns a default ExponentialBackOff
func newExponentialBackoff() *backoff.ExponentialBackOff {
	b := &backoff.ExponentialBackOff{
		InitialInterval:     50 * time.Millisecond,
		RandomizationFactor: backoff.DefaultRandomizationFactor,
		Multiplier:          backoff.DefaultMultiplier,
		MaxInterval:         maxRetryInterval,
		MaxElapsedTime:      maxElapsedTime,
		Stop:                backoff.Stop,
		Clock:               backoff.SystemClock,
	}
	b.Reset()
	return b
}
