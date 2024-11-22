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

// LogEmitter is a stanza operator that emits log entries to the consumer callback function `consumerFunc`
type LogEmitter struct {
	OutputOperator
	closeChan     chan struct{}
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
	apply(*LogEmitter)
}

func WithMaxBatchSize(maxBatchSize uint) EmitterOption {
	return maxBatchSizeOption{maxBatchSize}
}

type maxBatchSizeOption struct {
	maxBatchSize uint
}

func (o maxBatchSizeOption) apply(e *LogEmitter) {
	e.maxBatchSize = o.maxBatchSize
}

func WithFlushInterval(flushInterval time.Duration) EmitterOption {
	return flushIntervalOption{flushInterval}
}

type flushIntervalOption struct {
	flushInterval time.Duration
}

func (o flushIntervalOption) apply(e *LogEmitter) {
	e.flushInterval = o.flushInterval
}

// NewLogEmitter creates a new receiver output
func NewLogEmitter(set component.TelemetrySettings, consumerFunc func(context.Context, []*entry.Entry), opts ...EmitterOption) *LogEmitter {
	op, _ := NewOutputConfig("log_emitter", "log_emitter").Build(set)
	e := &LogEmitter{
		OutputOperator: op,
		closeChan:      make(chan struct{}),
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
func (e *LogEmitter) Start(_ operator.Persister) error {
	e.wg.Add(1)
	go e.flusher()
	return nil
}

// Stop will close the log channel and stop running goroutines
func (e *LogEmitter) Stop() error {
	e.stopOnce.Do(func() {
		close(e.closeChan)
		e.wg.Wait()
	})

	return nil
}

// ProcessBatch emits the entries to the consumerFunc
func (e *LogEmitter) ProcessBatch(ctx context.Context, entries []entry.Entry) error {
	for i := range entries {
		_ = e.Process(ctx, &entries[i])
	}

	return nil
}

// Process will emit an entry to the output channel
func (e *LogEmitter) Process(ctx context.Context, ent *entry.Entry) error {
	if oldBatch := e.appendEntry(ent); len(oldBatch) > 0 {
		e.consumerFunc(ctx, oldBatch)
	}

	return nil
}

// appendEntry appends the entry to the current batch. If maxBatchSize is reached, a new batch will be made, and the old batch
// (which should be flushed) will be returned
func (e *LogEmitter) appendEntry(ent *entry.Entry) []*entry.Entry {
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
func (e *LogEmitter) flusher() {
	defer e.wg.Done()

	ticker := time.NewTicker(e.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if oldBatch := e.makeNewBatch(); len(oldBatch) > 0 {
				e.consumerFunc(context.Background(), oldBatch)
			}
		case <-e.closeChan:
			// flush currently batched entries
			if oldBatch := e.makeNewBatch(); len(oldBatch) > 0 {
				e.consumerFunc(context.Background(), oldBatch)
			}
			return
		}
	}
}

// makeNewBatch replaces the current batch on the log emitter with a new batch, returning the old one
func (e *LogEmitter) makeNewBatch() []*entry.Entry {
	e.batchMux.Lock()
	defer e.batchMux.Unlock()

	if len(e.batch) == 0 {
		return nil
	}

	var oldBatch []*entry.Entry
	oldBatch, e.batch = e.batch, make([]*entry.Entry, 0, e.maxBatchSize)
	return oldBatch
}
