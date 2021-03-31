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

package recombine

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/antonmedv/expr"
	"github.com/antonmedv/expr/vm"

	"github.com/open-telemetry/opentelemetry-log-collection/entry"
	"github.com/open-telemetry/opentelemetry-log-collection/operator"
	"github.com/open-telemetry/opentelemetry-log-collection/operator/helper"
)

func init() {
	operator.Register("recombine", func() operator.Builder { return NewRecombineOperatorConfig("") })
}

// NewRecombineOperatorConfig creates a new recombine config with default values
func NewRecombineOperatorConfig(operatorID string) *RecombineOperatorConfig {
	return &RecombineOperatorConfig{
		TransformerConfig: helper.NewTransformerConfig(operatorID, "metadata"),
		MaxBatchSize:      1000,
		OverwriteWith:     "oldest",
	}
}

// RecombineOperatorConfig is the configuration of a recombine operator
type RecombineOperatorConfig struct {
	helper.TransformerConfig `yaml:",inline"`
	IsFirstEntry             string      `json:"is_first_entry" yaml:"is_first_entry"`
	IsLastEntry              string      `json:"is_last_entry"  yaml:"is_last_entry"`
	MaxBatchSize             int         `json:"max_batch_size" yaml:"max_batch_size"`
	CombineField             entry.Field `json:"combine_field"  yaml:"combine_field"`
	OverwriteWith            string      `json:"overwrite_with" yaml:"overwrite_with"`
}

// Build creates a new RecombineOperator from a config
func (c *RecombineOperatorConfig) Build(bc operator.BuildContext) ([]operator.Operator, error) {
	transformer, err := c.TransformerConfig.Build(bc)
	if err != nil {
		return nil, fmt.Errorf("failed to build transformer config: %s", err)
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
			return nil, fmt.Errorf("failed to compile is_first_entry: %s", err)
		}
	} else {
		matchesFirst = false
		prog, err = expr.Compile(c.IsLastEntry, expr.AsBool(), expr.AllowUndefinedVariables())
		if err != nil {
			return nil, fmt.Errorf("failed to compile is_last_entry: %s", err)
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

	recombine := &RecombineOperator{
		TransformerOperator: transformer,
		matchFirstLine:      matchesFirst,
		prog:                prog,
		maxBatchSize:        c.MaxBatchSize,
		overwriteWithOldest: overwriteWithOldest,
		batch:               make([]*entry.Entry, 0, c.MaxBatchSize),
		combineField:        c.CombineField,
	}

	return []operator.Operator{recombine}, nil
}

// RecombineOperator is an operator that combines a field from consecutive log entries
// into a single
type RecombineOperator struct {
	helper.TransformerOperator
	matchFirstLine      bool
	prog                *vm.Program
	maxBatchSize        int
	overwriteWithOldest bool
	combineField        entry.Field

	sync.Mutex
	batch []*entry.Entry
}

func (r *RecombineOperator) Start() error {
	return nil
}

func (r *RecombineOperator) Stop() error {
	r.Lock()
	defer r.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	r.flushUncombined(ctx)
	return nil
}

func (r *RecombineOperator) Process(ctx context.Context, e *entry.Entry) error {
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

	// This is the first entry in the next batch
	if matches && r.matchIndicatesFirst() {
		// Flush the existing batch
		err := r.flushCombined()
		if err != nil {
			return err
		}

		// Add the current log to the new batch
		r.addToBatch(ctx, e)
		return nil
	}

	// This is the last entry in a complete batch
	if matches && r.matchIndicatesLast() {
		r.addToBatch(ctx, e)
		err := r.flushCombined()
		if err != nil {
			return err
		}
		return nil
	}

	// This is neither the first entry of a new log,
	// nor the last entry of a log, so just add it to the batch
	r.addToBatch(ctx, e)
	return nil
}

func (r *RecombineOperator) matchIndicatesFirst() bool {
	return r.matchFirstLine
}

func (r *RecombineOperator) matchIndicatesLast() bool {
	return !r.matchFirstLine
}

// addToBatch adds the current entry to the current batch of entries that will be combined
func (r *RecombineOperator) addToBatch(_ context.Context, e *entry.Entry) {
	if len(r.batch) >= r.maxBatchSize {
		r.Error("Batch size exceeds max batch size. Flushing logs that have not been recombined")
		r.flushUncombined(context.Background())
	}

	r.batch = append(r.batch, e)
}

// flushUncombined flushes all the logs in the batch individually to the
// next output in the pipeline. This is only used when there is an error
// or at shutdown to avoid dropping the logs.
func (r *RecombineOperator) flushUncombined(ctx context.Context) {
	for _, entry := range r.batch {
		r.Write(ctx, entry)
	}
	r.batch = r.batch[:0]
}

// flushCombined combines the entries currently in the batch into a single entry,
// then forwards them to the next operator in the pipeline
func (r *RecombineOperator) flushCombined() error {
	// Skip flushing a combined log if the batch is empty
	if len(r.batch) == 0 {
		return nil
	}

	// Choose which entry we want to keep the rest of the fields from
	var base *entry.Entry
	if r.overwriteWithOldest {
		base = r.batch[0]
	} else {
		base = r.batch[len(r.batch)-1]
	}

	// Combine the combineField of each entry in the batch,
	// separated by newlines
	var recombined strings.Builder
	for i, e := range r.batch {
		var s string
		err := e.Read(r.combineField, &s)
		if err != nil {
			r.Errorf("entry does not contain the combine_field, so is being dropped")
			continue
		}

		recombined.WriteString(s)
		if i != len(r.batch)-1 {
			recombined.WriteByte('\n')
		}
	}

	// Set the recombined field on the entry
	base.Set(r.combineField, recombined.String())

	r.Write(context.Background(), base)
	r.batch = r.batch[:0]
	return nil
}
