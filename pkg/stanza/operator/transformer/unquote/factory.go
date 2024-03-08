// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package unquote // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/transformer/unquote"

import (
	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

var operatorType = component.MustNewType("unquote")

func init() {
	operator.RegisterFactory(NewFactory())
}

type factory struct{}

// NewFactory creates a factory.
func NewFactory() operator.Factory {
	return &factory{}
}

// Type gets the type of the operator.
func (f *factory) Type() component.Type {
	return operatorType
}

// NewDefaultConfig creates the default configuration.
func (f *factory) NewDefaultConfig(operatorID string) component.Config {
	return &Config{
		TransformerConfig: helper.NewTransformerConfig(operatorID, operatorType.String()),
	}
}

// CreateOperator creates an operator.
func (f *factory) CreateOperator(cfg component.Config, set component.TelemetrySettings) (operator.Operator, error) {
	c := cfg.(*Config)
	transformerOperator, err := helper.NewTransformer(c.TransformerConfig, set)
	if err != nil {
		return nil, err
	}

	return &Transformer{
		TransformerOperator: transformerOperator,
		field:               c.Field,
	}, nil
}
