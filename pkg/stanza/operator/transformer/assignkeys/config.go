// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package assignkeys // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/transformer/assignkeys"

import (
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/featuregate"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

const operatorType = "assign_keys"

var assignKeysTransformerFeatureGate = featuregate.GlobalRegistry().MustRegister(
	"logs.assignKeys",
	featuregate.StageBeta,
	featuregate.WithRegisterDescription("When enabled, allows usage of `assign_keys` transformer."),
	featuregate.WithRegisterReferenceURL("https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/30321"),
)

func init() {
	if assignKeysTransformerFeatureGate.IsEnabled() {
		operator.Register(operatorType, func() operator.Builder { return NewConfig() })
	}
}

// NewConfig creates a new assign_keys operator config with default values
func NewConfig() *Config {
	return NewConfigWithID(operatorType)
}

// NewConfigWithID creates a new assign_keys operator config with default values
func NewConfigWithID(operatorID string) *Config {
	return &Config{
		TransformerConfig: helper.NewTransformerConfig(operatorID, operatorType),
	}
}

// Config is the configuration of a assign_keys operator
type Config struct {
	helper.TransformerConfig `mapstructure:",squash"`
	Field                    entry.Field `mapstructure:"field"`
	Keys                     []string    `mapstructure:"keys"`
}

// Build will build an assign_keys operator from the supplied configuration
func (c Config) Build(set component.TelemetrySettings) (operator.Operator, error) {
	transformerOperator, err := c.TransformerConfig.Build(set)
	if err != nil {
		return nil, err
	}

	if len(c.Keys) == 0 {
		return nil, errors.New("assign_keys missing required field keys")
	}

	if _, ok := c.Field.FieldInterface.(entry.BodyField); ok {
		return &Transformer{
			TransformerOperator: transformerOperator,
			Field:               c.Field,
			Keys:                c.Keys,
		}, nil
	}

	if _, ok := c.Field.FieldInterface.(entry.ResourceField); ok {
		return &Transformer{
			TransformerOperator: transformerOperator,
			Field:               c.Field,
			Keys:                c.Keys,
		}, nil
	}

	if _, ok := c.Field.FieldInterface.(entry.AttributeField); ok {
		return &Transformer{
			TransformerOperator: transformerOperator,
			Field:               c.Field,
			Keys:                c.Keys,
		}, nil
	}

	return nil, fmt.Errorf("invalid field type: %T", c.Field.FieldInterface)
}
