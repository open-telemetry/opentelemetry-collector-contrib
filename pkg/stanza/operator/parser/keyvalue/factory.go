// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package keyvalue // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/parser/keyvalue"

import (
	"errors"

	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

var operatorType = component.MustNewType("key_value_parser")

func init() {
	operator.RegisterFactory(NewFactory())
}

// NewFactory creates a new factory.
func NewFactory() operator.Factory {
	return operator.NewFactory(operatorType, newDefaultConfig, createOperator)
}

func newDefaultConfig(operatorID string) component.Config {
	return &Config{
		ParserConfig: helper.NewParserConfig(operatorID, operatorType.String()),
		Delimiter:    "=",
	}
}

func createOperator(cfg component.Config, set component.TelemetrySettings) (operator.Operator, error) {
	c := cfg.(*Config)
	parserOperator, err := helper.NewParser(c.ParserConfig, set)
	if err != nil {
		return nil, err
	}

	if c.Delimiter == "" {
		return nil, errors.New("delimiter is a required parameter")
	}

	pairDelimiter := " "
	if c.PairDelimiter != "" {
		pairDelimiter = c.PairDelimiter
	}

	if c.Delimiter == pairDelimiter {
		return nil, errors.New("delimiter and pair_delimiter cannot be the same value")
	}

	return &Parser{
		ParserOperator: parserOperator,
		delimiter:      c.Delimiter,
		pairDelimiter:  pairDelimiter,
	}, nil
}
