// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package recombine // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/transformer/recombine"

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"time"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

const DefaultSourceIdentifier = "DefaultSourceIdentifier"

// Transformer is an operator that combines a field from consecutive log entries into a single
type Transformer struct {
	helper.TransformerOperator
	matchFirstLine        bool
	prog                  *vm.Program
	maxBatchSize          int
	maxUnmatchedBatchSize int
	maxSources            int
	overwriteWithNewest   bool
	combineField          entry.Field
	combineWith           string
	ticker                *time.Ticker
	forceFlushTimeout     time.Duration
	chClose               chan struct{}
	sourceIdentifier      entry.Field

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
	matchDetected          bool
}

func (t *Transformer) Start(_ operator.Persister) error {
	go t.flushLoop()
	return nil
}

func (t *Transformer) flushLoop() {
	for {
		select {
		case <-t.ticker.C:
			t.Lock()
			timeNow := time.Now()
			for source, batch := range t.batchMap {
				timeSinceFirstEntry := timeNow.Sub(batch.firstEntryObservedTime)
				if timeSinceFirstEntry < t.forceFlushTimeout {
					continue
				}
				if err := t.flushSource(context.Background(), source, t.Write); err != nil {
					t.Logger().Error("there was error flushing combined logs", zap.Error(err))
				}
			}
			// check every 1/5 forceFlushTimeout
			t.ticker.Reset(t.forceFlushTimeout / 5)
			t.Unlock()
		case <-t.chClose:
			t.ticker.Stop()
			return
		}
	}
}

func (t *Transformer) Stop() error {
	t.Lock()
	defer t.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	t.flushAllSources(ctx, t.Write)

	close(t.chClose)
	return nil
}

func (t *Transformer) ProcessBatch(ctx context.Context, entries []*entry.Entry) error {
	// Lock once for the entire batch to reduce overhead
	t.Lock()
	defer t.Unlock()

	var outputEntries []*entry.Entry
	var errs []error

	// Collect outputs instead of writing immediately
	collectWrite := func(_ context.Context, e *entry.Entry) error {
		outputEntries = append(outputEntries, e)
		return nil
	}

	for _, e := range entries {
		// Get the environment for executing the expression
		env := helper.GetExprEnv(e)

		m, err := expr.Run(t.prog, env)
		helper.PutExprEnv(env)
		if err != nil {
			if handleErr := t.HandleEntryErrorWithWrite(ctx, e, err, collectWrite); handleErr != nil {
				if !t.isQuietMode() {
					errs = append(errs, handleErr)
				}
			}
			continue
		}

		matches := m.(bool)
		var s string
		err = e.Read(t.sourceIdentifier, &s)
		if err != nil {
			t.Logger().Warn("entry does not contain the source_identifier, so it may be pooled with other sources")
			s = DefaultSourceIdentifier
		}

		if s == "" {
			s = DefaultSourceIdentifier
		}

		switch {
		case matches && t.matchFirstLine:
			// Flush the existing batch
			if err := t.flushSource(ctx, s, collectWrite); err != nil {
				errs = append(errs, err)
			}
			// Add the current log to the new batch
			t.addToBatch(ctx, e, s, matches, collectWrite)
		case matches && !t.matchFirstLine:
			t.addToBatch(ctx, e, s, matches, collectWrite)
			if err := t.flushSource(ctx, s, collectWrite); err != nil {
				errs = append(errs, err)
			}
		default:
			// Neither first nor last entry, just add to batch
			t.addToBatch(ctx, e, s, matches, collectWrite)
		}
	}

	// Write all collected outputs as a batch
	if len(outputEntries) > 0 {
		errs = append(errs, t.WriteBatch(ctx, outputEntries))
	}

	return errors.Join(errs...)
}

func (t *Transformer) Process(ctx context.Context, e *entry.Entry) error {
	// Lock the recombine operator because process can't run concurrently
	t.Lock()
	defer t.Unlock()

	// Get the environment for executing the expression.
	// In the future, we may want to provide access to the currently
	// batched entries so users can do comparisons to other entries
	// rather than just use absolute rules.
	env := helper.GetExprEnv(e)
	defer helper.PutExprEnv(env)

	m, err := expr.Run(t.prog, env)
	if err != nil {
		handleErr := t.HandleEntryError(ctx, e, err)
		if t.isQuietMode() {
			return nil
		}
		return handleErr
	}

	// this is guaranteed to be a boolean because of expr.AsBool
	matches := m.(bool)
	var s string
	err = e.Read(t.sourceIdentifier, &s)
	if err != nil {
		t.Logger().Warn("entry does not contain the source_identifier, so it may be pooled with other sources")
		s = DefaultSourceIdentifier
	}

	if s == "" {
		s = DefaultSourceIdentifier
	}

	switch {
	// This is the first entry in the next batch
	case matches && t.matchFirstLine:
		// Flush the existing batch
		if err := t.flushSource(ctx, s, t.Write); err != nil {
			return err
		}

		// Add the current log to the new batch
		t.addToBatch(ctx, e, s, matches, t.Write)
		return nil
	// This is the last entry in a complete batch
	case matches && !t.matchFirstLine:
		t.addToBatch(ctx, e, s, matches, t.Write)
		return t.flushSource(ctx, s, t.Write)
	}

	// This is neither the first entry of a new log,
	// nor the last entry of a log, so just add it to the batch
	t.addToBatch(ctx, e, s, matches, t.Write)
	return nil
}

// addToBatch adds the current entry to the current batch of entries that will be combined
func (t *Transformer) addToBatch(ctx context.Context, e *entry.Entry, source string, matches bool, write helper.WriteFunction) {
	batch, ok := t.batchMap[source]
	if !ok {
		if len(t.batchMap) >= t.maxSources {
			t.Logger().Error("Too many sources. Flushing all batched logs. Consider increasing max_sources parameter")
			t.flushAllSources(ctx, write)
		}
		batch = t.addNewBatch(source, e)
	} else {
		batch.numEntries++
		if t.overwriteWithNewest {
			batch.baseEntry = e
		}
	}

	// mark that match occurred to use max_unmatched_batch_size only when match didn't occur
	if matches && !batch.matchDetected {
		batch.matchDetected = true
	}

	// Combine the combineField of each entry in the batch,
	// separated by newlines
	var s string
	err := e.Read(t.combineField, &s)
	if err != nil {
		t.Logger().Error("entry does not contain the combine_field")
		return
	}
	if batch.recombined.Len() > 0 {
		batch.recombined.WriteString(t.combineWith)
	}
	batch.recombined.WriteString(s)

	if (t.maxLogSize > 0 && int64(batch.recombined.Len()) > t.maxLogSize) ||
		(t.maxBatchSize > 0 && batch.numEntries >= t.maxBatchSize) ||
		(!batch.matchDetected && t.maxUnmatchedBatchSize > 0 && batch.numEntries >= t.maxUnmatchedBatchSize) {
		if err := t.flushSource(ctx, source, write); err != nil {
			t.Logger().Error("there was error flushing combined logs", zap.Error(err))
		}
	}
}

// flushAllSources flushes all sources.
func (t *Transformer) flushAllSources(ctx context.Context, write helper.WriteFunction) {
	var errs error
	for source := range t.batchMap {
		errs = multierr.Append(errs, t.flushSource(ctx, source, write))
	}
	if errs != nil {
		t.Logger().Error("there was error flushing combined logs %s", zap.Error(errs))
	}
}

// flushSource combines the entries currently in the batch into a single entry,
// then forwards them to the next operator in the pipeline
func (t *Transformer) flushSource(ctx context.Context, source string, write helper.WriteFunction) error {
	batch := t.batchMap[source]
	// Skip flushing a combined log if the batch is empty
	if batch == nil {
		return nil
	}

	if batch.baseEntry == nil {
		t.removeBatch(source)
		return nil
	}

	// Set the recombined field on the entry
	err := batch.baseEntry.Set(t.combineField, batch.recombined.String())
	if err != nil {
		return err
	}

	err = write(ctx, batch.baseEntry)
	t.removeBatch(source)
	return err
}

// addNewBatch creates a new batch for the given source and adds the entry to it.
func (t *Transformer) addNewBatch(source string, e *entry.Entry) *sourceBatch {
	batch := t.batchPool.Get().(*sourceBatch)
	batch.baseEntry = e
	batch.numEntries = 1
	batch.recombined.Reset()
	batch.firstEntryObservedTime = e.ObservedTimestamp
	batch.matchDetected = false
	t.batchMap[source] = batch
	return batch
}

// removeBatch removes the batch for the given source.
func (t *Transformer) removeBatch(source string) {
	batch := t.batchMap[source]
	delete(t.batchMap, source)
	t.batchPool.Put(batch)
}

// isQuietMode returns true if the operator is configured to use quiet mode
func (t *Transformer) isQuietMode() bool {
	return t.OnError == helper.DropOnErrorQuiet || t.OnError == helper.SendOnErrorQuiet
}
