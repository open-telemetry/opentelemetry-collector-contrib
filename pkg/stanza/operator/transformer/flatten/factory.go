// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package flatten // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/transformer/flatten"

import (
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
	"go.opentelemetry.io/collector/component"
)

var operatorType = component.MustNewType("flatten")

func init() {
	operator.RegisterFactory(NewFactory())
}

type factory struct{}

// NewFactory creates a factory
func NewFactory() operator.Factory {
	return &factory{}
}

// Type gets the type of the operator
func (f *factory) Type() component.Type {
	return operatorType
}

// NewDefaultConfig creates the default configuration
func (f *factory) NewDefaultConfig(operatorID string) component.Config {
	return &Config{
		TransformerConfig: helper.NewTransformerConfig(operatorID, operatorType.String()),
	}
}

// CreateOperator creates a new Flatten operator
func (f *factory) CreateOperator(cfg component.Config, set component.TelemetrySettings) (operator.Operator, error) {
	c := cfg.(*Config)
	transformerOperator, err := helper.NewTransformer(c.TransformerConfig, set)
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
