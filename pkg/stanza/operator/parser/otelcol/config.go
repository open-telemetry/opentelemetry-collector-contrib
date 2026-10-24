// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelcol // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/parser/otelcol"

import (
	"go.opentelemetry.io/collector/component"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

const operatorType = "otelcol"

func init() {
	operator.Register(operatorType, func() operator.Builder { return NewConfig() })
}

// NewConfig creates a new otelcol parser config with default values.
func NewConfig() *Config {
	return NewConfigWithID(operatorType)
}

// NewConfigWithID creates a new otelcol parser config with default values.
func NewConfigWithID(operatorID string) *Config {
	return &Config{
		ParserConfig: helper.NewParserConfig(operatorID, operatorType),
	}
}

// Config is the configuration of an otelcol parser operator.
type Config struct {
	helper.ParserConfig `mapstructure:",squash"`
}

// Build will build an otelcol parser operator.
func (c Config) Build(set component.TelemetrySettings) (operator.Operator, error) {
	// Schema is fixed, so none of these are user-configurable.
	c.ParseFrom = entry.NewBodyField()
	c.ParseTo = entry.RootableField{Field: entry.NewAttributeField()}
	c.BodyField = nil
	c.TimeParser = nil
	c.SeverityConfig = nil
	c.TraceParser = nil
	c.ScopeNameParser = nil

	parserOperator, err := c.ParserConfig.Build(set)
	if err != nil {
		return nil, err
	}

	return &Parser{
		ParserOperator: parserOperator,
	}, nil
}
