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
		TransformerConfig: helper.NewTransformerConfig(operatorID, operatorType),
	}
}

// Config is the configuration of an otelcol parser operator.
//
// It embeds TransformerConfig rather than ParserConfig because the self-log
// schema is fixed: the fields a ParserConfig would expose (parse_from,
// parse_to, timestamp, severity, trace, scope_name) have exactly one correct
// value here, so they are not part of the config surface.
type Config struct {
	helper.TransformerConfig `mapstructure:",squash"`
}

// Build will build an otelcol parser operator.
func (c Config) Build(set component.TelemetrySettings) (operator.Operator, error) {
	transformerOperator, err := c.TransformerConfig.Build(set)
	if err != nil {
		return nil, err
	}

	return &Parser{
		ParserOperator: helper.ParserOperator{
			TransformerOperator: transformerOperator,
			ParseFrom:           entry.NewBodyField(),
			ParseTo:             entry.NewAttributeField(),
		},
	}, nil
}
