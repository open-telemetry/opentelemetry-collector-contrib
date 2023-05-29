// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package recombine // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/transformer/recombine"

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/antonmedv/expr"
	"github.com/antonmedv/expr/vm"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

const (
	operatorType       = "recombine"
	defaultCombineWith = "\n"
)

func init() {
	operator.Register(operatorType, func() operator.Builder { return NewConfig() })
}

// NewConfig creates a new recombine config with default values
func NewConfig() *Config {
	return NewConfigWithID(operatorType)
}

// NewConfigWithID creates a new recombine config with default values
func NewConfigWithID(operatorID string) *Config {
	return &Config{
		TransformerConfig: helper.NewTransformerConfig(operatorID, operatorType),
		MaxBatchSize:      1000,
		MaxSources:        1000,
		CombineWith:       defaultCombineWith,
		OverwriteWith:     "oldest",
		ForceFlushTimeout: 5 * time.Second,
		SourceIdentifier:  entry.NewAttributeField("file.path"),
	}
}

// Config is the configuration of a recombine operator
type Config struct {
	helper.TransformerConfig `mapstructure:",squash"`
	IsFirstEntry             string          `mapstructure:"is_first_entry"`
	IsLastEntry              string          `mapstructure:"is_last_entry"`
	MaxBatchSize             int             `mapstructure:"max_batch_size"`
	CombineField             entry.Field     `mapstructure:"combine_field"`
	CombineWith              string          `mapstructure:"combine_with"`
	SourceIdentifier         entry.Field     `mapstructure:"source_identifier"`
	OverwriteWith            string          `mapstructure:"overwrite_with"`
	ForceFlushTimeout        time.Duration   `mapstructure:"force_flush_period"`
	MaxSources               int             `mapstructure:"max_sources"`
	MaxLogSize               helper.ByteSize `mapstructure:"max_log_size,omitempty"`
}

// Build creates a new Transformer from a config
func (c *Config) Build(logger *zap.SugaredLogger) (operator.Operator, error) {
	transformer, err := c.TransformerConfig.Build(logger)
	if err != nil {
		return nil, fmt.Errorf("failed to build transformer config: %w", err)
	}

	if c.IsLastEntry != "" && c.IsFirstEntry != "" {
		return nil, fmt.Errorf("only one of is_first_entry and is_last_entry can be set")
	}

	if c.IsLastEntry == "" && c.IsFirstEntry == "" {
		return nil, fmt.Errorf("one of is_first_entry and is_last_entry must be set")
	}

	var matchesFirst bool
	var prog *vm.Program
	if c.IsFirstEntry != "" {
		matchesFirst = true
		prog, err = expr.Compile(c.IsFirstEntry, expr.AsBool(), expr.AllowUndefinedVariables())
		if err != nil {
			return nil, fmt.Errorf("failed to compile is_first_entry: %w", err)
		}
	} else {
		matchesFirst = false
		prog, err = expr.Compile(c.IsLastEntry, expr.AsBool(), expr.AllowUndefinedVariables())
		if err != nil {
			return nil, fmt.Errorf("failed to compile is_last_entry: %w", err)
		}
	}

	if c.CombineField.FieldInterface == nil {
		return nil, fmt.Errorf("missing required argument 'combine_field'")
	}

	var overwriteWithOldest bool
	switch c.OverwriteWith {
	case "newest":
		overwriteWithOldest = false
	case "oldest", "":
		overwriteWithOldest = true
	default:
		return nil, fmt.Errorf("invalid value '%s' for parameter 'overwrite_with'", c.OverwriteWith)
	}

	return &Transformer{
		TransformerOperator: transformer,
		matchFirstLine:      matchesFirst,
		prog:                prog,
		maxBatchSize:        c.MaxBatchSize,
		maxSources:          c.MaxSources,
		overwriteWithOldest: overwriteWithOldest,
		batchMap:            make(map[string]*sourceBatch),
		batchPool: sync.Pool{
			New: func() interface{} {
				return &sourceBatch{
					entries:    []*entry.Entry{},
					recombined: &bytes.Buffer{},
				}
			},
		},
		combineField:      c.CombineField,
		combineWith:       c.CombineWith,
		forceFlushTimeout: c.ForceFlushTimeout,
		ticker:            time.NewTicker(c.ForceFlushTimeout),
		chClose:           make(chan struct{}),
		sourceIdentifier:  c.SourceIdentifier,
		maxLogSize:        int64(c.MaxLogSize),
	}, nil
}

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
	entries                []*entry.Entry
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
				if err := r.flushSource(source, true); err != nil {
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
	r.flushUncombined(ctx)

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
	case matches && r.matchIndicatesFirst():
		// Flush the existing batch
		err := r.flushSource(s, true)
		if err != nil {
			return err
		}

		// Add the current log to the new batch
		r.addToBatch(ctx, e, s)
		return nil
	// This is the last entry in a complete batch
	case matches && r.matchIndicatesLast():
		fallthrough
	// When matching on first entry, never batch partial first. Just emit immediately
	case !matches && r.matchIndicatesFirst() && r.batchMap[s] == nil:
		r.addToBatch(ctx, e, s)
		return r.flushSource(s, true)
	}

	// This is neither the first entry of a new log,
	// nor the last entry of a log, so just add it to the batch
	r.addToBatch(ctx, e, s)
	return nil
}

func (r *Transformer) matchIndicatesFirst() bool {
	return r.matchFirstLine
}

func (r *Transformer) matchIndicatesLast() bool {
	return !r.matchFirstLine
}

// addToBatch adds the current entry to the current batch of entries that will be combined
func (r *Transformer) addToBatch(_ context.Context, e *entry.Entry, source string) {
	batch, ok := r.batchMap[source]
	if !ok {
		batch = r.addNewBatch(source, e)
		if len(r.batchMap) >= r.maxSources {
			r.Error("Batched source exceeds max source size. Flushing all batched logs. Consider increasing max_sources parameter")
			r.flushUncombined(context.Background())
			return
		}
	} else {
		// If the length of the batch is 0, this batch was flushed previously due to triggering size limit.
		// In this case, the firstEntryObservedTime should be updated to reset the timeout
		if len(batch.entries) == 0 {
			batch.firstEntryObservedTime = e.ObservedTimestamp
		}
		batch.entries = append(batch.entries, e)
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

	if (r.maxLogSize > 0 && int64(batch.recombined.Len()) > r.maxLogSize) || len(batch.entries) >= r.maxBatchSize {
		if err := r.flushSource(source, false); err != nil {
			r.Errorf("there was error flushing combined logs %s", err)
		}
	}

}

// flushUncombined flushes all the logs in the batch individually to the
// next output in the pipeline. This is only used when there is an error
// or at shutdown to avoid dropping the logs.
func (r *Transformer) flushUncombined(ctx context.Context) {
	for source := range r.batchMap {
		for _, entry := range r.batchMap[source].entries {
			r.Write(ctx, entry)
		}
		r.removeBatch(source)
	}
	r.ticker.Reset(r.forceFlushTimeout)
}

// flushSource combines the entries currently in the batch into a single entry,
// then forwards them to the next operator in the pipeline
func (r *Transformer) flushSource(source string, deleteSource bool) error {
	batch := r.batchMap[source]
	// Skip flushing a combined log if the batch is empty
	if batch == nil {
		return nil
	}

	if len(batch.entries) == 0 {
		r.removeBatch(source)
		return nil
	}

	// Choose which entry we want to keep the rest of the fields from
	var base *entry.Entry
	entries := batch.entries

	if r.overwriteWithOldest {
		base = entries[0]
	} else {
		base = entries[len(entries)-1]
	}

	// Set the recombined field on the entry
	err := base.Set(r.combineField, batch.recombined.String())
	if err != nil {
		return err
	}

	r.Write(context.Background(), base)
	if deleteSource {
		r.removeBatch(source)
	} else {
		batch.entries = batch.entries[:0]
		batch.recombined.Reset()
	}

	return nil
}

// addNewBatch creates a new batch for the given source and adds the entry to it.
func (r *Transformer) addNewBatch(source string, e *entry.Entry) *sourceBatch {
	batch := r.batchPool.Get().(*sourceBatch)
	batch.entries = append(batch.entries[:0], e)
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
