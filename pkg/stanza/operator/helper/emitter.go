// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package helper // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"

import (
	"context"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
)

type LogEmitter interface {
	operator.Operator
	Start(operator.Persister) error
	Stop() error
	ProcessBatch(context.Context, []*entry.Entry) error
	Process(context.Context, *entry.Entry) error
}

// BatchingLogEmitter is a stanza operator that emits log entries to the consumer callback function `consumerFunc` with batching
type BatchingLogEmitter struct {
	OutputOperator
	cancel        context.CancelFunc
	stopOnce      sync.Once
	batchMux      sync.Mutex
	batch         []*entry.Entry
	wg            sync.WaitGroup
	maxBatchSize  uint
	flushInterval time.Duration
	consumerFunc  func(context.Context, []*entry.Entry)
}

var (
	defaultFlushInterval      = 100 * time.Millisecond
	defaultMaxBatchSize  uint = 100
)

type EmitterOption interface {
	apply(*BatchingLogEmitter)
}

func WithMaxBatchSize(maxBatchSize uint) EmitterOption {
	return maxBatchSizeOption{maxBatchSize}
}

type maxBatchSizeOption struct {
	maxBatchSize uint
}

func (o maxBatchSizeOption) apply(e *BatchingLogEmitter) {
	e.maxBatchSize = o.maxBatchSize
}

func WithFlushInterval(flushInterval time.Duration) EmitterOption {
	return flushIntervalOption{flushInterval}
}

type flushIntervalOption struct {
	flushInterval time.Duration
}

func (o flushIntervalOption) apply(e *BatchingLogEmitter) {
	e.flushInterval = o.flushInterval
}

// NewBatchingLogEmitter creates a new receiver output
func NewBatchingLogEmitter(set component.TelemetrySettings, consumerFunc func(context.Context, []*entry.Entry), opts ...EmitterOption) *BatchingLogEmitter {
	op, _ := NewOutputConfig("batching_log_emitter", "batching_log_emitter").Build(set)
	e := &BatchingLogEmitter{
		OutputOperator: op,
		maxBatchSize:   defaultMaxBatchSize,
		batch:          make([]*entry.Entry, 0, defaultMaxBatchSize),
		flushInterval:  defaultFlushInterval,
		consumerFunc:   consumerFunc,
	}
	for _, opt := range opts {
		opt.apply(e)
	}
	return e
}

// Start starts the goroutine(s) required for this operator
func (e *BatchingLogEmitter) Start(_ operator.Persister) error {
	ctx, cancel := context.WithCancel(context.Background())
	e.cancel = cancel

	e.wg.Add(1)
	go e.flusher(ctx)
	return nil
}

// Stop will close the log channel and stop running goroutines
func (e *BatchingLogEmitter) Stop() error {
	e.stopOnce.Do(func() {
		// the cancel func could be nil if the emitter is never started.
		if e.cancel != nil {
			e.cancel()
		}
		e.wg.Wait()
	})

	return nil
}

// ProcessBatch emits the entries to the consumerFunc
func (e *BatchingLogEmitter) ProcessBatch(ctx context.Context, entries []*entry.Entry) error {
	if oldBatch := e.appendEntries(entries); len(oldBatch) > 0 {
		e.consumerFunc(ctx, oldBatch)
	}

	return nil
}

// appendEntries appends the entry to the current batch. If maxBatchSize is reached, a new batch will be made, and the old batch
// (which should be flushed) will be returned
func (e *BatchingLogEmitter) appendEntries(entries []*entry.Entry) []*entry.Entry {
	e.batchMux.Lock()
	defer e.batchMux.Unlock()

	e.batch = append(e.batch, entries...)
	if uint(len(e.batch)) >= e.maxBatchSize {
		var oldBatch []*entry.Entry
		oldBatch, e.batch = e.batch, make([]*entry.Entry, 0, e.maxBatchSize)
		return oldBatch
	}

	return nil
}

// Process will emit an entry to the consumerFunc
func (e *BatchingLogEmitter) Process(ctx context.Context, ent *entry.Entry) error {
	if oldBatch := e.appendEntry(ent); len(oldBatch) > 0 {
		e.consumerFunc(ctx, oldBatch)
	}

	return nil
}

// appendEntry appends the entry to the current batch. If maxBatchSize is reached, a new batch will be made, and the old batch
// (which should be flushed) will be returned
func (e *BatchingLogEmitter) appendEntry(ent *entry.Entry) []*entry.Entry {
	e.batchMux.Lock()
	defer e.batchMux.Unlock()

	e.batch = append(e.batch, ent)
	if uint(len(e.batch)) >= e.maxBatchSize {
		var oldBatch []*entry.Entry
		oldBatch, e.batch = e.batch, make([]*entry.Entry, 0, e.maxBatchSize)
		return oldBatch
	}

	return nil
}

// flusher flushes the current batch every flush interval. Intended to be run as a goroutine
func (e *BatchingLogEmitter) flusher(ctx context.Context) {
	defer e.wg.Done()

	ticker := time.NewTicker(e.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if oldBatch := e.makeNewBatch(); len(oldBatch) > 0 {
				e.consumerFunc(ctx, oldBatch)
			}
		case <-ctx.Done():
			// Create a new context with timeout for the final flush
			flushCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			// flush currently batched entries
			if oldBatch := e.makeNewBatch(); len(oldBatch) > 0 {
				e.consumerFunc(flushCtx, oldBatch)
			}
			return
		}
	}
}

// makeNewBatch replaces the current batch on the log emitter with a new batch, returning the old one
func (e *BatchingLogEmitter) makeNewBatch() []*entry.Entry {
	e.batchMux.Lock()
	defer e.batchMux.Unlock()

	if len(e.batch) == 0 {
		return nil
	}

	var oldBatch []*entry.Entry
	oldBatch, e.batch = e.batch, make([]*entry.Entry, 0, e.maxBatchSize)
	return oldBatch
}

// SynchronousLogEmitter is a stanza operator that emits log entries to the consumer callback function `consumerFunc` synchronously
type SynchronousLogEmitter struct {
	OutputOperator
	consumerFunc func(context.Context, []*entry.Entry)
}

func NewSynchronousLogEmitter(set component.TelemetrySettings, consumerFunc func(context.Context, []*entry.Entry)) *SynchronousLogEmitter {
	op, _ := NewOutputConfig("synchronous_log_emitter", "synchronous_log_emitter").Build(set)
	return &SynchronousLogEmitter{
		OutputOperator: op,
		consumerFunc:   consumerFunc,
	}
}

func (e *SynchronousLogEmitter) Start(_ operator.Persister) error {
	return nil
}

func (e *SynchronousLogEmitter) Stop() error {
	return nil
}

func (e *SynchronousLogEmitter) ProcessBatch(ctx context.Context, entries []*entry.Entry) error {
	e.consumerFunc(ctx, entries)
	return nil
}

func (e *SynchronousLogEmitter) Process(ctx context.Context, ent *entry.Entry) error {
	e.consumerFunc(ctx, []*entry.Entry{ent})
	return nil
}
