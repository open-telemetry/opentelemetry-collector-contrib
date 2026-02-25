// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package retain // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/transformer/retain"

import (
	"errors"
	"strings"

	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

const operatorType = "retain"

func init() {
	operator.Register(operatorType, func() operator.Builder { return NewConfig() })
}

// NewConfig creates a new retain operator config with default values
func NewConfig() *Config {
	return NewConfigWithID(operatorType)
}

// NewConfigWithID creates a new retain operator config with default values
func NewConfigWithID(operatorID string) *Config {
	return &Config{
		TransformerConfig: helper.NewTransformerConfig(operatorID, operatorType),
	}
}

// Config is the configuration of a retain operator
type Config struct {
	helper.TransformerConfig `mapstructure:",squash"`
	Fields                   []entry.Field `mapstructure:"fields"`
}

// Build will build a retain operator from the supplied configuration
func (c Config) Build(set component.TelemetrySettings) (operator.Operator, error) {
	transformerOperator, err := c.TransformerConfig.Build(set)
	if err != nil {
		return nil, err
	}
	if len(c.Fields) == 0 {
		return nil, errors.New("retain: 'fields' is empty")
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
