// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package flatten // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/transformer/flatten"

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/errors"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

const operatorType = "flatten"

func init() {
	operator.Register(operatorType, func() operator.Builder { return NewConfig() })
}

// NewConfig creates a new flatten operator config with default values
func NewConfig() *Config {
	return NewConfigWithID(operatorType)
}

// NewConfigWithID creates a new flatten operator config with default values
func NewConfigWithID(operatorID string) *Config {
	return &Config{
		TransformerConfig: helper.NewTransformerConfig(operatorID, operatorType),
	}
}

// Config is the configuration of a flatten operator
type Config struct {
	helper.TransformerConfig `mapstructure:",squash"`
	Field                    entry.Field `mapstructure:"field"`
}

// Build will build a Flatten operator from the supplied configuration
func (c Config) Build(logger *zap.SugaredLogger) (operator.Operator, error) {
	transformerOperator, err := c.TransformerConfig.Build(logger)
	if err != nil {
		return nil, err
	}

	if e, ok := c.Field.FieldInterface.(entry.BodyField); ok {
		return &Transformer[entry.BodyField]{
			TransformerOperator: transformerOperator,
			Field:               e,
		}, nil
	}

	if e, ok := c.Field.FieldInterface.(entry.ResourceField); ok {
		return &Transformer[entry.ResourceField]{
			TransformerOperator: transformerOperator,
			Field:               e,
		}, nil
	}

	if e, ok := c.Field.FieldInterface.(entry.AttributeField); ok {
		return &Transformer[entry.AttributeField]{
			TransformerOperator: transformerOperator,
			Field:               e,
		}, nil
	}

	return nil, fmt.Errorf("invalid field type: %T", c.Field.FieldInterface)
}

// Transformer flattens an object in the entry field
type Transformer[T interface {
	entry.BodyField | entry.ResourceField | entry.AttributeField
	entry.FieldInterface
	Parent() T
	Child(string) T
}] struct {
	helper.TransformerOperator
	Field T
}

// Process will process an entry with a flatten transformation.
func (p *Transformer[T]) Process(ctx context.Context, entry *entry.Entry) error {
	return p.ProcessWith(ctx, entry, p.Transform)
}

// Transform will apply the flatten operation to an entry
func (p *Transformer[T]) Transform(entry *entry.Entry) error {
	parent := p.Field.Parent()
	val, ok := entry.Delete(p.Field)
	if !ok {
		// The field doesn't exist, so ignore it
		return fmt.Errorf("apply flatten: field %s does not exist on entry", p.Field)
	}

	valMap, ok := val.(map[string]any)
	if !ok {
		// The field we were asked to flatten was not a map, so put it back
		err := entry.Set(p.Field, val)
		if err != nil {
			return errors.Wrap(err, "reset non-map field")
		}
		return fmt.Errorf("apply flatten: field %s is not a map", p.Field)
	}

	for k, v := range valMap {
		err := entry.Set(parent.Child(k), v)
		if err != nil {
			return err
		}
	}
	return nil
}
