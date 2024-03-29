// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package recombine // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/transformer/recombine"

import (
	"bytes"
	"fmt"
	"sync"
	"time"

	"github.com/expr-lang/expr/vm"
	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

var operatorType = component.MustNewType("recombine")

func init() {
	operator.RegisterFactory(NewFactory())
}

// NewFactory creates a new factory.
func NewFactory() operator.Factory {
	return operator.NewFactory(operatorType, newDefaultConfig, createOperator)
}

func newDefaultConfig(operatorID string) component.Config {
	return &Config{
		TransformerConfig: helper.NewTransformerConfig(operatorID, operatorType.String()),
		MaxBatchSize:      1000,
		MaxSources:        1000,
		CombineWith:       defaultCombineWith,
		OverwriteWith:     "newest",
		ForceFlushTimeout: 5 * time.Second,
		SourceIdentifier:  entry.NewAttributeField("file.path"),
	}
}

func createOperator(cfg component.Config, set component.TelemetrySettings) (operator.Operator, error) {
	c := cfg.(*Config)
	transformer, err := helper.NewTransformer(c.TransformerConfig, set)
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
		prog, err = helper.ExprCompileBool(c.IsFirstEntry)
		if err != nil {
			return nil, fmt.Errorf("failed to compile is_first_entry: %w", err)
		}
	} else {
		matchesFirst = false
		prog, err = helper.ExprCompileBool(c.IsLastEntry)
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
