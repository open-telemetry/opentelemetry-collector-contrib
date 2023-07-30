// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package unquote // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/transformer/unquote"

import (
	"context"
	"fmt"
	"strconv"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

const operatorType = "unquote"

func init() {
	operator.Register(operatorType, func() operator.Builder { return NewConfig() })
}

// NewConfig creates a new unquote config with default values
func NewConfig() *Config {
	return NewConfigWithID(operatorType)
}

// NewConfigWithID creates a new unquote config with default values
func NewConfigWithID(operatorID string) *Config {
	return &Config{
		TransformerConfig: helper.NewTransformerConfig(operatorID, operatorType),
	}
}

// Config is the configuration of an unquote operator.
type Config struct {
	helper.TransformerConfig `mapstructure:",squash"`
	Field                    entry.Field `mapstructure:"field"`
}

// Build will build an unquote operator.
func (c Config) Build(logger *zap.SugaredLogger) (operator.Operator, error) {
	transformerOperator, err := c.TransformerConfig.Build(logger)
	if err != nil {
		return nil, err
	}

	return &Transformer{
		TransformerOperator: transformerOperator,
		field:               c.Field,
	}, nil
}

// Transformer is an operator that unquotes a string.
type Transformer struct {
	helper.TransformerOperator
	field entry.Field
}

// Process will unquote a string
func (t *Transformer) Process(ctx context.Context, entry *entry.Entry) error {
	return t.ProcessWith(ctx, entry, t.unquote)
}

// unquote will unquote a string
func (t *Transformer) unquote(e *entry.Entry) error {
	value, ok := t.field.Get(e)
	if !ok {
		return nil
	}

	switch v := value.(type) {
	case string:
		s, err := strconv.Unquote(v)
		if err != nil {
			return err
		}
		return t.field.Set(e, s)
	default:
		return fmt.Errorf("type %T cannot be unquoted", value)
	}
}
