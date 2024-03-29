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

// NewFactory creates a new factory.
func NewFactory() operator.Factory {
	return operator.NewFactory(operatorType, newDefaultConfig, createOperator)
}

func newDefaultConfig(operatorID string) component.Config {
	return &Config{
		TransformerConfig: helper.NewTransformerConfig(operatorID, operatorType.String()),
	}
}

func createOperator(cfg component.Config, set component.TelemetrySettings) (operator.Operator, error) {
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
