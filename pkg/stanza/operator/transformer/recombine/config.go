// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package recombine // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/transformer/recombine"

import (
	"bytes"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/expr-lang/expr/vm"
	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/attrs"
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
		TransformerConfig:     helper.NewTransformerConfig(operatorID, operatorType),
		MaxBatchSize:          1000,
		MaxUnmatchedBatchSize: 100,
		MaxSources:            1000,
		CombineWith:           defaultCombineWith,
		OverwriteWith:         "oldest",
		ForceFlushTimeout:     5 * time.Second,
		SourceIdentifier:      entry.NewAttributeField(attrs.LogFilePath),
	}
}

// Config is the configuration of a recombine operator
type Config struct {
	helper.TransformerConfig `mapstructure:",squash"`
	IsFirstEntry             string          `mapstructure:"is_first_entry"`
	IsLastEntry              string          `mapstructure:"is_last_entry"`
	MaxBatchSize             int             `mapstructure:"max_batch_size"`
	MaxUnmatchedBatchSize    int             `mapstructure:"max_unmatched_batch_size"`
	CombineField             entry.Field     `mapstructure:"combine_field"`
	CombineWith              string          `mapstructure:"combine_with"`
	SourceIdentifier         entry.Field     `mapstructure:"source_identifier"`
	OverwriteWith            string          `mapstructure:"overwrite_with"`
	ForceFlushTimeout        time.Duration   `mapstructure:"force_flush_period"`
	MaxSources               int             `mapstructure:"max_sources"`
	MaxLogSize               helper.ByteSize `mapstructure:"max_log_size,omitempty"`
	States                   []State         `mapstructure:"states"`
}

// Build creates a new Transformer from a config
func (c *Config) Build(set component.TelemetrySettings) (operator.Operator, error) {
	transformer, err := c.TransformerConfig.Build(set)
	if err != nil {
		return nil, fmt.Errorf("failed to build transformer config: %w", err)
	}

	if (c.IsFirstEntry != "" && c.IsLastEntry != "") ||
		(c.IsFirstEntry != "" && len(c.States) > 0) ||
		(c.IsLastEntry != "" && len(c.States) > 0) {
		return nil, errors.New("only one of is_first_entry, is_last_entry or states can be set")
	}

	if c.IsLastEntry == "" && c.IsFirstEntry == "" && len(c.States) == 0 {
		return nil, errors.New("one of is_first_entry, is_last_entry or states must be set")
	}

	var stateMachine bool
	var matchesFirst bool
	var compiledStates []CompiledState
	var prog *vm.Program
	if c.IsFirstEntry != "" {
		matchesFirst = true
		stateMachine = false
		prog, err = helper.ExprCompileBool(c.IsFirstEntry)
		if err != nil {
			return nil, fmt.Errorf("failed to compile is_first_entry: %w", err)
		}
	} else if c.IsLastEntry != "" {
		matchesFirst = false
		stateMachine = false
		prog, err = helper.ExprCompileBool(c.IsLastEntry)
		if err != nil {
			return nil, fmt.Errorf("failed to compile is_last_entry: %w", err)
		}
	} else if len(c.States) > 0 {
		stateMachine = true
		for _, state := range c.States {
			prog, err = helper.ExprCompileBool(state.Condition)
			if err != nil {
				return nil, fmt.Errorf("failed to compile state '%s' condition: %w", state.Name, err)
			}
			compiledStates = append(compiledStates, CompiledState{Name: state.Name, Prog: prog, Cont: state.Cont})
		}
	}

	if c.CombineField.FieldInterface == nil {
		return nil, errors.New("missing required argument 'combine_field'")
	}

	var overwriteWithNewest bool
	switch c.OverwriteWith {
	case "newest":
		overwriteWithNewest = true
	case "oldest", "":
		overwriteWithNewest = false
	default:
		return nil, fmt.Errorf("invalid value '%s' for parameter 'overwrite_with'", c.OverwriteWith)
	}

	return &Transformer{
		TransformerOperator:   transformer,
		matchFirstLine:        matchesFirst,
		stateMachineEnabled:   stateMachine,
		compiledStates:        compiledStates,
		currentState:          "start_state",
		prog:                  prog,
		maxBatchSize:          c.MaxBatchSize,
		maxUnmatchedBatchSize: c.MaxUnmatchedBatchSize,
		maxSources:            c.MaxSources,
		overwriteWithNewest:   overwriteWithNewest,
		batchMap:              make(map[string]*sourceBatch),
		batchPool: sync.Pool{
			New: func() any {
				return &sourceBatch{
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
