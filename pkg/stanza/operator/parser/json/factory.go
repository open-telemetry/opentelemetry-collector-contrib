// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package json // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/parser/json"

import (
	jsoniter "github.com/json-iterator/go"
	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

var operatorType = component.MustNewType("json_parser")

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
		ParserConfig: helper.NewParserConfig(operatorID, operatorType.String()),
	}
}

// CreateOperator creates a parser.
func (f *factory) CreateOperator(cfg component.Config, set component.TelemetrySettings) (operator.Operator, error) {
	c := cfg.(*Config)
	parserOperator, err := helper.NewParser(c.ParserConfig, set)
	if err != nil {
		return nil, err
	}

	return &Parser{
		ParserOperator: parserOperator,
		json:           jsoniter.ConfigFastest,
	}, nil
}
