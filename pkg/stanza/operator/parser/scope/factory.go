// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package scope // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/parser/scope"

import (
	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

var operatorType = component.MustNewType("scope_name_parser")

func init() {
	operator.RegisterFactory(NewFactory())
}

type factory struct{}

// New creates a new regex parser config with default values
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
		ScopeNameParser:   helper.NewScopeNameParser(),
	}
}

// CreateOperator creates a parser.
func (f *factory) CreateOperator(cfg component.Config, set component.TelemetrySettings) (operator.Operator, error) {
	c := cfg.(*Config)
	transformerOperator, err := helper.NewTransformer(c.TransformerConfig, set)
	if err != nil {
		return nil, err
	}

	return &Parser{
		TransformerOperator: transformerOperator,
		ScopeNameParser:     c.ScopeNameParser,
	}, nil
}
