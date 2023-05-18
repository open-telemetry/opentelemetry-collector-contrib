// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package retain // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/transformer/retain"

import (
	"context"
	"fmt"
	"strings"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

const operatorType = "retain"

func init() {
	operator.Register(operatorType, func() operator.Builder { return NewConfig() })
}

// NewConfig creates a new retain operator config with default values
func NewConfig() *Config {
	return NewConfigWithID(operatorType)
}

// NewConfigWithID creates a new retain operator config with default values
func NewConfigWithID(operatorID string) *Config {
	return &Config{
		TransformerConfig: helper.NewTransformerConfig(operatorID, operatorType),
	}
}

// Config is the configuration of a retain operator
type Config struct {
	helper.TransformerConfig `mapstructure:",squash"`
	Fields                   []entry.Field `mapstructure:"fields"`
}

// Build will build a retain operator from the supplied configuration
func (c Config) Build(logger *zap.SugaredLogger) (operator.Operator, error) {
	transformerOperator, err := c.TransformerConfig.Build(logger)
	if err != nil {
		return nil, err
	}
	if c.Fields == nil || len(c.Fields) == 0 {
		return nil, fmt.Errorf("retain: 'fields' is empty")
	}

	retainOp := &Transformer{
		TransformerOperator: transformerOperator,
		Fields:              c.Fields,
	}

	for _, field := range c.Fields {
		typeCheck := field.String()
		if strings.HasPrefix(typeCheck, "resource") {
			retainOp.AllResourceFields = true
			continue
		}
		if strings.HasPrefix(typeCheck, "attributes") {
			retainOp.AllAttributeFields = true
			continue
		}
		retainOp.AllBodyFields = true
	}
	return retainOp, nil
}

// Transformer keeps the given fields and deletes the rest.
type Transformer struct {
	helper.TransformerOperator
	Fields             []entry.Field
	AllBodyFields      bool
	AllAttributeFields bool
	AllResourceFields  bool
}

// Process will process an entry with a retain transformation.
func (p *Transformer) Process(ctx context.Context, entry *entry.Entry) error {
	return p.ProcessWith(ctx, entry, p.Transform)
}

// Transform will apply the retain operation to an entry
func (p *Transformer) Transform(e *entry.Entry) error {
	newEntry := entry.New()
	newEntry.ObservedTimestamp = e.ObservedTimestamp
	newEntry.Timestamp = e.Timestamp

	if !p.AllResourceFields {
		newEntry.Resource = e.Resource
	}
	if !p.AllAttributeFields {
		newEntry.Attributes = e.Attributes
	}
	if !p.AllBodyFields {
		newEntry.Body = e.Body
	}

	for _, field := range p.Fields {
		val, ok := e.Get(field)
		if !ok {
			continue
		}
		err := newEntry.Set(field, val)
		if err != nil {
			return err
		}
	}

	*e = *newEntry
	return nil
}
