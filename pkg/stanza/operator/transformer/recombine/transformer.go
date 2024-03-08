// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package recombine // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/transformer/recombine"

import (
	"bytes"
	"context"
	"sync"
	"time"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

// Transformer is an operator that combines a field from consecutive log entries
// into a single
type Transformer struct {
	helper.TransformerOperator
	matchFirstLine      bool
	prog                *vm.Program
	maxBatchSize        int
	maxSources          int
	overwriteWithOldest bool
	combineField        entry.Field
	combineWith         string
	ticker              *time.Ticker
	forceFlushTimeout   time.Duration
	chClose             chan struct{}
	sourceIdentifier    entry.Field

	sync.Mutex
	batchPool  sync.Pool
	batchMap   map[string]*sourceBatch
	maxLogSize int64
}

// sourceBatch contains the status info of a batch
type sourceBatch struct {
	baseEntry              *entry.Entry
	numEntries             int
	recombined             *bytes.Buffer
	firstEntryObservedTime time.Time
}

func (r *Transformer) Start(_ operator.Persister) error {
	go r.flushLoop()

	return nil
}

func (r *Transformer) flushLoop() {
	for {
		select {
		case <-r.ticker.C:
			r.Lock()
			timeNow := time.Now()
			for source, batch := range r.batchMap {
				timeSinceFirstEntry := timeNow.Sub(batch.firstEntryObservedTime)
				if timeSinceFirstEntry < r.forceFlushTimeout {
					continue
				}
				if err := r.flushSource(context.Background(), source); err != nil {
					r.Errorf("there was error flushing combined logs %s", err)
				}
			}
			// check every 1/5 forceFlushTimeout
			r.ticker.Reset(r.forceFlushTimeout / 5)
			r.Unlock()
		case <-r.chClose:
			r.ticker.Stop()
			return
		}
	}
}

func (r *Transformer) Stop() error {
	r.Lock()
	defer r.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	r.flushAllSources(ctx)

	close(r.chClose)
	return nil
}

const DefaultSourceIdentifier = "DefaultSourceIdentifier"

func (r *Transformer) Process(ctx context.Context, e *entry.Entry) error {
	// Lock the recombine operator because process can't run concurrently
	r.Lock()
	defer r.Unlock()

	// Get the environment for executing the expression.
	// In the future, we may want to provide access to the currently
	// batched entries so users can do comparisons to other entries
	// rather than just use absolute rules.
	env := helper.GetExprEnv(e)
	defer helper.PutExprEnv(env)

	m, err := expr.Run(r.prog, env)
	if err != nil {
		return r.HandleEntryError(ctx, e, err)
	}

	// this is guaranteed to be a boolean because of expr.AsBool
	matches := m.(bool)
	var s string
	err = e.Read(r.sourceIdentifier, &s)
	if err != nil {
		r.Warn("entry does not contain the source_identifier, so it may be pooled with other sources")
		s = DefaultSourceIdentifier
	}

	if s == "" {
		s = DefaultSourceIdentifier
	}

	switch {
	// This is the first entry in the next batch
	case matches && r.matchFirstLine:
		// Flush the existing batch
		if err := r.flushSource(ctx, s); err != nil {
			return err
		}

		// Add the current log to the new batch
		r.addToBatch(ctx, e, s)
		return nil
	// This is the last entry in a complete batch
	case matches && !r.matchFirstLine:
		r.addToBatch(ctx, e, s)
		return r.flushSource(ctx, s)
	}

	// This is neither the first entry of a new log,
	// nor the last entry of a log, so just add it to the batch
	r.addToBatch(ctx, e, s)
	return nil
}

// addToBatch adds the current entry to the current batch of entries that will be combined
func (r *Transformer) addToBatch(ctx context.Context, e *entry.Entry, source string) {
	batch, ok := r.batchMap[source]
	if !ok {
		if len(r.batchMap) >= r.maxSources {
			r.Error("Too many sources. Flushing all batched logs. Consider increasing max_sources parameter")
			r.flushAllSources(ctx)
		}
		batch = r.addNewBatch(source, e)
	} else {
		batch.numEntries++
		if r.overwriteWithOldest {
			batch.baseEntry = e
		}
	}

	// Combine the combineField of each entry in the batch,
	// separated by newlines
	var s string
	err := e.Read(r.combineField, &s)
	if err != nil {
		r.Errorf("entry does not contain the combine_field")
		return
	}
	if batch.recombined.Len() > 0 {
		batch.recombined.WriteString(r.combineWith)
	}
	batch.recombined.WriteString(s)

	if (r.maxLogSize > 0 && int64(batch.recombined.Len()) > r.maxLogSize) || batch.numEntries >= r.maxBatchSize {
		if err := r.flushSource(ctx, source); err != nil {
			r.Errorf("there was error flushing combined logs %s", err)
		}
	}
}

// flushAllSources flushes all sources.
func (r *Transformer) flushAllSources(ctx context.Context) {
	var errs []error
	for source := range r.batchMap {
		if err := r.flushSource(ctx, source); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		r.Errorf("there was error flushing combined logs %s", errs)
	}
}

// flushSource combines the entries currently in the batch into a single entry,
// then forwards them to the next operator in the pipeline
func (r *Transformer) flushSource(ctx context.Context, source string) error {
	batch := r.batchMap[source]
	// Skip flushing a combined log if the batch is empty
	if batch == nil {
		return nil
	}

	if batch.baseEntry == nil {
		r.removeBatch(source)
		return nil
	}

	// Set the recombined field on the entry
	err := batch.baseEntry.Set(r.combineField, batch.recombined.String())
	if err != nil {
		return err
	}

	r.Write(ctx, batch.baseEntry)
	r.removeBatch(source)
	return nil
}

// addNewBatch creates a new batch for the given source and adds the entry to it.
func (r *Transformer) addNewBatch(source string, e *entry.Entry) *sourceBatch {
	batch := r.batchPool.Get().(*sourceBatch)
	batch.baseEntry = e
	batch.numEntries = 1
	batch.recombined.Reset()
	batch.firstEntryObservedTime = e.ObservedTimestamp
	r.batchMap[source] = batch
	return batch
}

// removeBatch removes the batch for the given source.
func (r *Transformer) removeBatch(source string) {
	batch := r.batchMap[source]
	delete(r.batchMap, source)
	r.batchPool.Put(batch)
}
