// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sanitizeutf8 // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/transformer/sanitizeutf8"

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

const (
	operatorType           = "sanitize_utf8"
	invalidCharacterMarker = string(utf8.RuneError) // Unicode replacement character
)

func init() {
	operator.Register(operatorType, func() operator.Builder { return NewConfig() })
}

// NewConfig creates a new sanitize_utf8 config with default values
func NewConfig() *Config {
	return NewConfigWithID(operatorType)
}

// NewConfigWithID creates a new sanitize_utf8 config with default values
func NewConfigWithID(operatorID string) *Config {
	return &Config{
		TransformerConfig: helper.NewTransformerConfig(operatorID, operatorType),
		Field:             entry.NewBodyField(),
	}
}

// Config is the configuration of a sanitize_utf8 operator
type Config struct {
	helper.TransformerConfig `mapstructure:",squash"`
	Field                    entry.Field `mapstructure:"field"`
}

// Build creates a new Transformer from a config
func (c *Config) Build(set component.TelemetrySettings) (operator.Operator, error) {
	transformer, err := c.TransformerConfig.Build(set)
	if err != nil {
		return nil, fmt.Errorf("failed to build transformer config: %w", err)
	}

	return &Transformer{
		TransformerOperator: transformer,
		field:               c.Field,
	}, nil
}

// Transformer is an operator that replaces invalid UTF-8 characters in a field
type Transformer struct {
	helper.TransformerOperator
	field entry.Field
}

func (t *Transformer) ProcessBatch(ctx context.Context, entries []*entry.Entry) error {
	return t.ProcessBatchWithTransform(ctx, entries, t.ProcessWith)
}

func (t *Transformer) Process(ctx context.Context, e *entry.Entry) error {
	return t.TransformerOperator.ProcessWith(ctx, e, t.ProcessWith)
}

func (t *Transformer) ProcessWith(e *entry.Entry) error {
	v, ok := t.field.Get(e)
	if !ok {
		return nil
	}
	s, ok := v.(string)
	if !ok {
		return fmt.Errorf("field '%s' is not a string, got %T", t.field.String(), v)
	}
	if utf8.ValidString(s) {
		return nil
	}
	if err := t.field.Set(e, strings.ToValidUTF8(s, invalidCharacterMarker)); err != nil {
		return fmt.Errorf("failed to set field %s: %w", t.field.String(), err)
	}
	return nil
}
