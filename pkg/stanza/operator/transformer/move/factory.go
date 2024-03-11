// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package move // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/transformer/move"

import (
	"fmt"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
	"go.opentelemetry.io/collector/component"
)

var operatorType = component.MustNewType("move")

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

// CreateOperator creates a new move operator
func (f *factory) CreateOperator(cfg component.Config, set component.TelemetrySettings) (operator.Operator, error) {
	c := cfg.(*Config)
	transformerOperator, err := helper.NewTransformer(c.TransformerConfig, set)
	if err != nil {
		return nil, err
	}

	if c.To == entry.NewNilField() || c.From == entry.NewNilField() {
		return nil, fmt.Errorf("move: missing to or from field")
	}

	return &Transformer{
		TransformerOperator: transformerOperator,
		From:                c.From,
		To:                  c.To,
	}, nil
}
