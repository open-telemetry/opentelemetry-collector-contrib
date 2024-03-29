// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package assignkeys // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/transformer/assignkeys"

import (
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/featuregate"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

var assignKeysTransformerFeatureGate = featuregate.GlobalRegistry().MustRegister(
	"logs.assignKeys",
	featuregate.StageAlpha,
	featuregate.WithRegisterDescription("When enabled, allows usage of `assign_keys` transformer."),
	featuregate.WithRegisterReferenceURL("https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/30321"),
)

var operatorType = component.MustNewType("assign_keys")

func init() {
	if assignKeysTransformerFeatureGate.IsEnabled() {
		operator.RegisterFactory(NewFactory())
	}
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

	if len(c.Keys) == 0 {
		return nil, fmt.Errorf("assign_keys missing required field keys")
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
