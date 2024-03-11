// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package retain // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/transformer/retain"

import (
	"fmt"
	"strings"

	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

var operatorType = component.MustNewType("retain")

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

// CreateOperator creates an operator
func (f *factory) CreateOperator(cfg component.Config, set component.TelemetrySettings) (operator.Operator, error) {
	c := cfg.(*Config)
	transformerOperator, err := helper.NewTransformer(c.TransformerConfig, set)
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
