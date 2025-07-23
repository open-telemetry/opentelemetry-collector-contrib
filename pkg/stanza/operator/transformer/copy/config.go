// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package copy // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/transformer/copy"

import (
	"errors"

	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

const operatorType = "copy"

var (
	errMissingFrom = errors.New("copy: missing from field")
	errMissingTo   = errors.New("copy: missing to field")
)

func init() {
	operator.Register(operatorType, func() operator.Builder { return NewConfig() })
}

// NewConfig creates a new copy operator config with default values
func NewConfig() *Config {
	return NewConfigWithID(operatorType)
}

// NewConfigWithID creates a new copy operator config with default values
func NewConfigWithID(operatorID string) *Config {
	return &Config{
		TransformerConfig: helper.NewTransformerConfig(operatorID, operatorType),
	}
}

// Config is the configuration of a copy operator
type Config struct {
	helper.TransformerConfig `mapstructure:",squash"`
	From                     entry.Field `mapstructure:"from"`
	To                       entry.Field `mapstructure:"to"`
}

// Build will build a copy operator from the supplied configuration
func (c Config) Build(set component.TelemetrySettings) (operator.Operator, error) {
	transformerOperator, err := c.TransformerConfig.Build(set)
	if err != nil {
		return nil, err
	}

	validationErrs := []error{}

	if c.From.IsEmpty() {
		validationErrs = append(validationErrs, errMissingFrom)
	}

	if c.To.IsEmpty() {
		validationErrs = append(validationErrs, errMissingTo)
	}

	if len(validationErrs) > 0 {
		return nil, errors.Join(validationErrs...)
	}

	return &Transformer{
		TransformerOperator: transformerOperator,
		From:                c.From,
		To:                  c.To,
	}, nil
}
